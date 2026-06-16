package streamproc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

type ValueExtractor func(record *Record) (float64, error)

type windowBucket struct {
	ID         string
	Start      time.Time
	End        time.Time
	StartSeq   int64
	EndSeq     int64
	Sum        float64
	Count      int64
	Min        float64
	Max        float64
	RecordIDs  []string
	records    []*Record
}

func newWindowBucket(id string, start, end time.Time) *windowBucket {
	return &windowBucket{
		ID:        id,
		Start:     start,
		End:       end,
		Min:       math.Inf(1),
		Max:       math.Inf(-1),
		RecordIDs: make([]string, 0),
		records:   make([]*Record, 0),
	}
}

func (b *windowBucket) add(rec *Record, value float64) {
	b.Sum += value
	b.Count++
	if value < b.Min {
		b.Min = value
	}
	if value > b.Max {
		b.Max = value
	}
	if rec.ID != "" {
		b.RecordIDs = append(b.RecordIDs, rec.ID)
	}
	if b.StartSeq == 0 || rec.SeqID < b.StartSeq {
		b.StartSeq = rec.SeqID
	}
	if rec.SeqID > b.EndSeq {
		b.EndSeq = rec.SeqID
	}
	b.records = append(b.records, rec)
}

type WindowAggregator struct {
	seqCounter    int64
	closedWindows int64
	countSize     int64
	countSlide    int64

	name          string
	windowType    WindowType
	aggregation   AggregationType
	size          time.Duration
	slide         time.Duration
	extractor     ValueExtractor

	buckets      map[string]*windowBucket
	bucketOrder  []string
	mu           sync.RWMutex

	results      chan *WindowResult
	resultBuffer int

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	ticker    *time.Ticker
	watermark time.Duration
}

type WindowConfig struct {
	WindowType     WindowType
	Aggregation    AggregationType
	Size           time.Duration
	Slide          time.Duration
	CountSize      int64
	CountSlide     int64
	Extractor      ValueExtractor
	ResultBuffer   int
	Watermark      time.Duration
}

func NewWindowAggregator(name string, cfg WindowConfig) (*WindowAggregator, error) {
	if cfg.WindowType == WindowTypeTumblingTime || cfg.WindowType == WindowTypeSlidingTime {
		if cfg.Size <= 0 {
			return nil, ErrInvalidWindowSize
		}
		if cfg.Slide <= 0 {
			cfg.Slide = cfg.Size
		}
	} else {
		if cfg.CountSize <= 0 {
			return nil, ErrInvalidWindowSize
		}
		if cfg.CountSlide <= 0 {
			cfg.CountSlide = cfg.CountSize
		}
	}
	if cfg.ResultBuffer <= 0 {
		cfg.ResultBuffer = 100
	}
	if cfg.Watermark < 0 {
		cfg.Watermark = 0
	}

	return &WindowAggregator{
		name:         name,
		windowType:   cfg.WindowType,
		aggregation:  cfg.Aggregation,
		size:         cfg.Size,
		slide:        cfg.Slide,
		countSize:    cfg.CountSize,
		countSlide:   cfg.CountSlide,
		extractor:    cfg.Extractor,
		buckets:      make(map[string]*windowBucket),
		bucketOrder:  make([]string, 0),
		results:      make(chan *WindowResult, cfg.ResultBuffer),
		resultBuffer: cfg.ResultBuffer,
		watermark:    cfg.Watermark,
	}, nil
}

func (w *WindowAggregator) Name() string {
	return w.name
}

func (w *WindowAggregator) Results() <-chan *WindowResult {
	return w.results
}

func (w *WindowAggregator) Start(ctx context.Context) {
	w.ctx, w.cancel = context.WithCancel(ctx)
	if w.windowType == WindowTypeTumblingTime || w.windowType == WindowTypeSlidingTime {
		w.wg.Add(1)
		w.ticker = time.NewTicker(w.size / 4)
		if w.ticker == nil {
			w.ticker = time.NewTicker(100 * time.Millisecond)
		}
		go w.timeWindowLoop()
	}
}

func (w *WindowAggregator) timeWindowLoop() {
	defer w.wg.Done()
	defer w.ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.ticker.C:
			w.checkTimeWindows()
		}
	}
}

func (w *WindowAggregator) checkTimeWindows() {
	now := time.Now().Add(-w.watermark)
	w.mu.Lock()
	defer w.mu.Unlock()

	toClose := make([]string, 0)
	for _, id := range w.bucketOrder {
		b := w.buckets[id]
		if !b.End.After(now) {
			toClose = append(toClose, id)
		}
	}

	for _, id := range toClose {
		w.closeBucketLocked(id)
	}
}

func (w *WindowAggregator) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.windowType == WindowTypeTumblingTime || w.windowType == WindowTypeSlidingTime {
		for _, id := range w.bucketOrder {
			w.closeBucketLocked(id)
		}
	}
	close(w.results)
}

func (w *WindowAggregator) getTimeBucketID(t time.Time) string {
	truncated := t.Truncate(w.size)
	if w.windowType == WindowTypeSlidingTime {
		truncated = t.Truncate(w.slide)
	}
	return fmt.Sprintf("win-%d", truncated.UnixNano())
}

func (w *WindowAggregator) getOrCreateTimeBucket(t time.Time) *windowBucket {
	now := time.Now()
	var windows []time.Time

	if w.windowType == WindowTypeTumblingTime {
		start := t.Truncate(w.size)
		windows = []time.Time{start}
	} else {
		earliestStart := t.Add(-w.size + w.slide).Truncate(w.slide)
		for start := earliestStart; !start.After(t); start = start.Add(w.slide) {
			if start.Add(w.size).After(t) {
				windows = append(windows, start)
			}
		}
		if len(windows) == 0 {
			start := t.Truncate(w.slide)
			windows = []time.Time{start}
		}
	}

	var result *windowBucket
	for _, start := range windows {
		id := fmt.Sprintf("win-%d", start.UnixNano())
		b, ok := w.buckets[id]
		if !ok {
			b = newWindowBucket(id, start, start.Add(w.size))
			w.buckets[id] = b
			w.bucketOrder = append(w.bucketOrder, id)
		}
		_ = now
		result = b
	}
	return result
}

func (w *WindowAggregator) getCountBucketID(seq int64) string {
	if w.windowType == WindowTypeTumblingCount {
		bucketIdx := (seq - 1) / w.countSize
		return fmt.Sprintf("win-c-%d", bucketIdx)
	}
	bucketIdx := (seq - 1) / w.countSlide
	return fmt.Sprintf("win-c-%d", bucketIdx)
}

func (w *WindowAggregator) getOrCreateCountBuckets(seq int64) []*windowBucket {
	buckets := make([]*windowBucket, 0)

	if w.windowType == WindowTypeTumblingCount {
		bucketIdx := (seq - 1) / w.countSize
		id := fmt.Sprintf("win-c-%d", bucketIdx)
		b, ok := w.buckets[id]
		if !ok {
			startSeq := bucketIdx*w.countSize + 1
			endSeq := startSeq + w.countSize - 1
			b = &windowBucket{
				ID:       id,
				StartSeq: startSeq,
				EndSeq:   endSeq,
				Min:      math.Inf(1),
				Max:      math.Inf(-1),
			}
			w.buckets[id] = b
			w.bucketOrder = append(w.bucketOrder, id)
		}
		buckets = append(buckets, b)
	} else {
		startIdx := int64(0)
		if seq > w.countSize {
			startIdx = (seq - w.countSize) / w.countSlide
			if startIdx < 0 {
				startIdx = 0
			}
		}
		endIdx := (seq - 1) / w.countSlide

		for i := startIdx; i <= endIdx; i++ {
			id := fmt.Sprintf("win-c-%d", i)
			startSeq := i*w.countSlide + 1
			endSeq := startSeq + w.countSize - 1
			if seq >= startSeq && seq <= endSeq {
				b, ok := w.buckets[id]
				if !ok {
					b = &windowBucket{
						ID:       id,
						StartSeq: startSeq,
						EndSeq:   endSeq,
						Min:      math.Inf(1),
						Max:      math.Inf(-1),
					}
					w.buckets[id] = b
					w.bucketOrder = append(w.bucketOrder, id)
				}
				buckets = append(buckets, b)
			}
		}

		if len(buckets) == 0 {
			id := fmt.Sprintf("win-c-%d", endIdx)
			startSeq := endIdx*w.countSlide + 1
			endSeq := startSeq + w.countSize - 1
			b := &windowBucket{
				ID:       id,
				StartSeq: startSeq,
				EndSeq:   endSeq,
				Min:      math.Inf(1),
				Max:      math.Inf(-1),
			}
			w.buckets[id] = b
			w.bucketOrder = append(w.bucketOrder, id)
			buckets = append(buckets, b)
		}
	}

	return buckets
}

func (w *WindowAggregator) Process(ctx context.Context, rec *Record) ([]*Record, error) {
	if rec == nil {
		return nil, nil
	}

	value := 1.0
	if w.extractor != nil {
		var err error
		value, err = w.extractor(rec)
		if err != nil {
			return nil, fmt.Errorf("window aggregator '%s': extract value: %w", w.name, err)
		}
	}

	atomic.AddInt64(&w.seqCounter, 1)

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.windowType == WindowTypeTumblingTime || w.windowType == WindowTypeSlidingTime {
		ts := rec.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		w.getTimeBucketID(ts)
		if w.windowType == WindowTypeTumblingTime {
			id := w.getTimeBucketID(ts)
			b, ok := w.buckets[id]
			if !ok {
				start := ts.Truncate(w.size)
				b = newWindowBucket(id, start, start.Add(w.size))
				w.buckets[id] = b
				w.bucketOrder = append(w.bucketOrder, id)
			}
			b.add(rec, value)
		} else {
			buckets := w.getSlidingTimeBuckets(ts)
			for _, b := range buckets {
				b.add(rec, value)
			}
		}
		w.checkTimeWindowsLocked()
	} else {
		seq := rec.SeqID
		if seq <= 0 {
			seq = atomic.LoadInt64(&w.seqCounter)
		}
		buckets := w.getOrCreateCountBuckets(seq)
		for _, b := range buckets {
			b.add(rec, value)
		}
		w.checkCountWindowsLocked(seq)
	}

	return []*Record{rec}, nil
}

func (w *WindowAggregator) getSlidingTimeBuckets(t time.Time) []*windowBucket {
	buckets := make([]*windowBucket, 0)
	earliestStart := t.Add(-w.size + w.slide).Truncate(w.slide)
	for start := earliestStart; !start.After(t); start = start.Add(w.slide) {
		end := start.Add(w.size)
		if !t.Before(start) && t.Before(end) {
			id := fmt.Sprintf("win-%d", start.UnixNano())
			b, ok := w.buckets[id]
			if !ok {
				b = newWindowBucket(id, start, end)
				w.buckets[id] = b
				w.bucketOrder = append(w.bucketOrder, id)
			}
			buckets = append(buckets, b)
		}
	}
	if len(buckets) == 0 {
		start := t.Truncate(w.slide)
		end := start.Add(w.size)
		id := fmt.Sprintf("win-%d", start.UnixNano())
		b, ok := w.buckets[id]
		if !ok {
			b = newWindowBucket(id, start, end)
			w.buckets[id] = b
			w.bucketOrder = append(w.bucketOrder, id)
		}
		buckets = append(buckets, b)
	}
	return buckets
}

func (w *WindowAggregator) checkTimeWindowsLocked() {
	now := time.Now().Add(-w.watermark)
	toClose := make([]string, 0)
	for _, id := range w.bucketOrder {
		b := w.buckets[id]
		if !b.End.After(now) {
			toClose = append(toClose, id)
		}
	}
	for _, id := range toClose {
		w.closeBucketLocked(id)
	}
}

func (w *WindowAggregator) checkCountWindowsLocked(currentSeq int64) {
	toClose := make([]string, 0)
	for _, id := range w.bucketOrder {
		b := w.buckets[id]
		if w.windowType == WindowTypeTumblingCount {
			if b.Count >= w.countSize && b.EndSeq > 0 {
				toClose = append(toClose, id)
			}
		} else {
			if currentSeq >= b.EndSeq && b.Count > 0 {
				toClose = append(toClose, id)
			}
		}
	}
	for _, id := range toClose {
		w.closeBucketLocked(id)
	}
}

func (w *WindowAggregator) closeBucketLocked(id string) {
	b, ok := w.buckets[id]
	if !ok {
		return
	}

	if b.Count > 0 {
		result := &WindowResult{
			WindowID:    b.ID,
			WindowType:  w.windowType,
			Start:       b.Start,
			End:         b.End,
			StartSeq:    b.StartSeq,
			EndSeq:      b.EndSeq,
			Aggregation: w.aggregation,
			Count:       b.Count,
			RecordIDs:   b.RecordIDs,
		}

		switch w.aggregation {
		case AggregationSum:
			result.Value = b.Sum
		case AggregationCount:
			result.Value = float64(b.Count)
		case AggregationAvg:
			if b.Count > 0 {
				result.Value = b.Sum / float64(b.Count)
			}
		case AggregationMin:
			if b.Count > 0 {
				result.Value = b.Min
			}
		case AggregationMax:
			if b.Count > 0 {
				result.Value = b.Max
			}
		}

		select {
		case w.results <- result:
		default:
		}
		atomic.AddInt64(&w.closedWindows, 1)
	}

	delete(w.buckets, id)
	for i, bid := range w.bucketOrder {
		if bid == id {
			w.bucketOrder = append(w.bucketOrder[:i], w.bucketOrder[i+1:]...)
			break
		}
	}
}

func (w *WindowAggregator) FlushAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range append([]string{}, w.bucketOrder...) {
		w.closeBucketLocked(id)
	}
}

func (w *WindowAggregator) ActiveWindowCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.buckets)
}

func (w *WindowAggregator) ClosedWindowCount() int64 {
	return atomic.LoadInt64(&w.closedWindows)
}

type windowState struct {
	SeqCounter    int64             `json:"seq_counter"`
	ClosedWindows int64             `json:"closed_windows"`
	Buckets       []windowBucketState `json:"buckets"`
}

type windowBucketState struct {
	ID        string    `json:"id"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	StartSeq  int64     `json:"start_seq"`
	EndSeq    int64     `json:"end_seq"`
	Sum       float64   `json:"sum"`
	Count     int64     `json:"count"`
	Min       float64   `json:"min"`
	Max       float64   `json:"max"`
	RecordIDs []string  `json:"record_ids"`
}

func (w *WindowAggregator) SaveState() ([]byte, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	buckets := make([]windowBucketState, 0, len(w.bucketOrder))
	for _, id := range w.bucketOrder {
		b := w.buckets[id]
		buckets = append(buckets, windowBucketState{
			ID:        b.ID,
			Start:     b.Start,
			End:       b.End,
			StartSeq:  b.StartSeq,
			EndSeq:    b.EndSeq,
			Sum:       b.Sum,
			Count:     b.Count,
			Min:       b.Min,
			Max:       b.Max,
			RecordIDs: b.RecordIDs,
		})
	}

	state := windowState{
		SeqCounter:    atomic.LoadInt64(&w.seqCounter),
		ClosedWindows: atomic.LoadInt64(&w.closedWindows),
		Buckets:       buckets,
	}
	return json.Marshal(state)
}

func (w *WindowAggregator) RestoreState(data []byte) error {
	var state windowState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("window aggregator '%s' restore state: %w", w.name, err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	atomic.StoreInt64(&w.seqCounter, state.SeqCounter)
	atomic.StoreInt64(&w.closedWindows, state.ClosedWindows)
	w.buckets = make(map[string]*windowBucket)
	w.bucketOrder = make([]string, 0, len(state.Buckets))

	for _, bs := range state.Buckets {
		b := &windowBucket{
			ID:        bs.ID,
			Start:     bs.Start,
			End:       bs.End,
			StartSeq:  bs.StartSeq,
			EndSeq:    bs.EndSeq,
			Sum:       bs.Sum,
			Count:     bs.Count,
			Min:       bs.Min,
			Max:       bs.Max,
			RecordIDs: bs.RecordIDs,
			records:   make([]*Record, 0),
		}
		w.buckets[bs.ID] = b
		w.bucketOrder = append(w.bucketOrder, bs.ID)
	}
	return nil
}
