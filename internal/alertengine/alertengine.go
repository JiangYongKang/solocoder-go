package alertengine

import (
	"fmt"
	"sync"
	"time"
)

const maxHistorySize = 1000

var validLevels = map[AlertLevel]bool{
	LevelInfo:     true,
	LevelWarning:  true,
	LevelAlert:    true,
	LevelCritical: true,
}

var validOperators = map[ComparisonOperator]bool{
	OpGreaterThan:        true,
	OpLessThan:           true,
	OpGreaterThanOrEqual: true,
	OpLessThanOrEqual:    true,
}

var validCompareTypes = map[CompareType]bool{
	CompareRingbi: true,
	CompareTongbi: true,
}

var validDurationTypes = map[DurationType]bool{
	DurationByCount: true,
	DurationByTime:  true,
}

var validSilentTypes = map[SilentType]bool{
	SilentDaily: true,
	SilentRange: true,
}

type Engine struct {
	mu        sync.RWMutex
	cfg       EngineConfig
	rules     map[string]*AlertRule
	states    map[string]*RuleState
	notifiers map[string]Notifier
}

func NewEngine(cfg EngineConfig) *Engine {
	if cfg.Notifiers == nil {
		cfg.Notifiers = make(map[string]Notifier)
	}
	if cfg.DefaultInhibitDuration <= 0 {
		cfg.DefaultInhibitDuration = 5 * time.Minute
	}
	return &Engine{
		cfg:       cfg,
		rules:     make(map[string]*AlertRule),
		states:    make(map[string]*RuleState),
		notifiers: cfg.Notifiers,
	}
}

func (e *Engine) RegisterNotifier(notifier Notifier) error {
	if notifier == nil {
		return ErrInvalidCondition
	}
	if notifier.Name() == "" {
		return ErrInvalidRule
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.notifiers[notifier.Name()] = notifier
	return nil
}

func (e *Engine) AddRule(rule *AlertRule) error {
	if rule == nil || rule.ID == "" {
		return ErrInvalidRule
	}
	if rule.Name == "" {
		return ErrInvalidRule
	}
	if rule.Threshold == nil && rule.RingbiTongbi == nil {
		return ErrNoConditionDefined
	}

	if rule.InitialLevel == "" {
		rule.InitialLevel = LevelAlert
	}
	if rule.InhibitDuration < 0 {
		rule.InhibitDuration = e.cfg.DefaultInhibitDuration
	}

	if !validLevels[rule.InitialLevel] {
		return ErrInvalidLevel
	}

	if rule.Threshold != nil {
		if !validOperators[rule.Threshold.Operator] {
			return ErrInvalidOperator
		}
	}

	if rule.RingbiTongbi != nil {
		if !validCompareTypes[rule.RingbiTongbi.CompareType] {
			return ErrInvalidCondition
		}
		if rule.RingbiTongbi.PercentThreshold < 0 {
			return ErrInvalidThreshold
		}
		if rule.RingbiTongbi.Period <= 0 {
			return ErrInvalidDuration
		}
	}

	if rule.Duration != nil {
		if !validDurationTypes[rule.Duration.Type] {
			return ErrInvalidDuration
		}
		if rule.Duration.Type == DurationByCount && rule.Duration.CheckCount <= 0 {
			return ErrInvalidDuration
		}
		if rule.Duration.Type == DurationByTime && rule.Duration.TimeWindow <= 0 {
			return ErrInvalidDuration
		}
	}

	for _, sw := range rule.SilentWindows {
		if !validSilentTypes[sw.Type] {
			return ErrInvalidSilentWindow
		}
		if sw.Type == SilentDaily {
			if _, _, err := parseTimeStr(sw.StartTime); err != nil {
				return ErrInvalidSilentWindow
			}
			if _, _, err := parseTimeStr(sw.EndTime); err != nil {
				return ErrInvalidSilentWindow
			}
		}
		if sw.Type == SilentRange {
			if sw.StartDate.IsZero() || sw.EndDate.IsZero() {
				return ErrInvalidSilentWindow
			}
			if !sw.EndDate.After(sw.StartDate) {
				return ErrInvalidSilentWindow
			}
		}
	}

	for _, esc := range rule.Escalations {
		if !validLevels[esc.FromLevel] || !validLevels[esc.ToLevel] {
			return ErrInvalidLevel
		}
		if esc.AfterDuration <= 0 {
			return ErrInvalidDuration
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.rules[rule.ID]; exists {
		return ErrRuleAlreadyExists
	}
	e.rules[rule.ID] = rule
	e.states[rule.ID] = &RuleState{
		Alert: &AlertState{
			RuleID:       rule.ID,
			Status:       StatusPending,
			CurrentLevel: rule.InitialLevel,
		},
	}
	return nil
}

func (e *Engine) RemoveRule(ruleID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.rules[ruleID]; !exists {
		return ErrRuleNotFound
	}
	delete(e.rules, ruleID)
	delete(e.states, ruleID)
	return nil
}

func (e *Engine) GetRule(ruleID string) (*AlertRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rule, exists := e.rules[ruleID]
	if !exists {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

func (e *Engine) GetAlertState(ruleID string) (*AlertState, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	state, exists := e.states[ruleID]
	if !exists {
		return nil, ErrRuleNotFound
	}
	return state.Alert, nil
}

func (e *Engine) Evaluate(ruleID string, dataPoint MetricDataPoint) error {
	if dataPoint.Timestamp.IsZero() {
		return ErrInvalidMetricData
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	rule, exists := e.rules[ruleID]
	if !exists {
		return ErrRuleNotFound
	}
	state := e.states[ruleID]
	alert := state.Alert

	alert.LastEvaluatedTime = time.Now()
	alert.HistoryValues = append(alert.HistoryValues, dataPoint)
	if len(alert.HistoryValues) > maxHistorySize {
		alert.HistoryValues = alert.HistoryValues[len(alert.HistoryValues)-maxHistorySize:]
	}

	baseConditionMet := false
	var triggerValue float64

	if rule.Threshold != nil {
		met, val, err := evaluateThreshold(rule.Threshold, dataPoint)
		if err != nil {
			return err
		}
		baseConditionMet = met
		triggerValue = val
	} else if rule.RingbiTongbi != nil {
		met, val, err := evaluateRingbiTongbi(rule.RingbiTongbi, dataPoint, alert.HistoryValues)
		if err != nil {
			return err
		}
		baseConditionMet = met
		triggerValue = val
	}

	now := time.Now()

	if baseConditionMet {
		alert.ConsecutiveHits++
		if alert.FirstFiredTime.IsZero() {
			alert.FirstFiredTime = now
		}
		alert.LastFiredTime = now
	} else {
		alert.ConsecutiveHits = 0
		alert.FirstFiredTime = time.Time{}
	}

	conditionMet := baseConditionMet
	if rule.Duration != nil {
		conditionMet = evaluateDuration(rule.Duration, baseConditionMet, alert, dataPoint, now)
	}

	if conditionMet {
		alert.TriggerValue = triggerValue
		alert.TriggerTime = now

		wasTriggered := alert.Status == StatusFiring || alert.Status == StatusSuppressed
		if !wasTriggered {
			alert.Status = StatusFiring
			alert.CurrentLevel = rule.InitialLevel
			alert.FirstTriggeredTime = now
		}

		escalated, newLevel := checkEscalation(rule.Escalations, alert, now)
		if escalated {
			alert.CurrentLevel = newLevel
		}

		if !isInhibitPeriod(rule, alert, now) && !isInSilentPeriod(rule, alert, now) {
			e.sendNotificationsLocked(rule, alert, triggerValue, now)
			alert.LastNotifiedTime = now
		}
	} else {
		if alert.Status == StatusFiring || alert.Status == StatusSuppressed {
			alert.Status = StatusResolved
			alert.ResolvedTime = now
			alert.FirstTriggeredTime = time.Time{}
		}
	}

	return nil
}

func evaluateThreshold(cond *ThresholdCondition, dp MetricDataPoint) (bool, float64, error) {
	if cond == nil {
		return false, 0, ErrInvalidCondition
	}
	switch cond.Operator {
	case OpGreaterThan:
		return dp.Value > cond.Threshold, dp.Value, nil
	case OpLessThan:
		return dp.Value < cond.Threshold, dp.Value, nil
	case OpGreaterThanOrEqual:
		return dp.Value >= cond.Threshold, dp.Value, nil
	case OpLessThanOrEqual:
		return dp.Value <= cond.Threshold, dp.Value, nil
	default:
		return false, 0, ErrInvalidOperator
	}
}

func evaluateRingbiTongbi(cond *RingbiTongbiCondition, current MetricDataPoint, history []MetricDataPoint) (bool, float64, error) {
	if cond == nil {
		return false, 0, ErrInvalidCondition
	}
	if cond.PercentThreshold < 0 {
		return false, 0, ErrInvalidThreshold
	}

	var compareValue float64
	var found bool

	targetOffset := cond.Period
	if cond.CompareType == CompareTongbi {
		targetOffset = 365 * 24 * time.Hour
	}

	tolerance := cond.Tolerance
	if tolerance <= 0 {
		if cond.CompareType == CompareTongbi {
			tolerance = 24 * time.Hour
		} else {
			tolerance = cond.Period / 2
		}
	}

	targetTime := current.Timestamp.Add(-targetOffset)

	for i := len(history) - 1; i >= 0; i-- {
		dp := history[i]
		diff := dp.Timestamp.Sub(targetTime)
		if diff < 0 {
			diff = -diff
		}
		if diff <= tolerance {
			compareValue = dp.Value
			found = true
			break
		}
	}

	if !found || compareValue == 0 {
		return false, current.Value, nil
	}

	percentChange := ((current.Value - compareValue) / compareValue) * 100
	if percentChange < 0 {
		percentChange = -percentChange
	}

	return percentChange >= cond.PercentThreshold, current.Value, nil
}

func evaluateDuration(cond *DurationCondition, currentConditionMet bool, alert *AlertState, dp MetricDataPoint, now time.Time) bool {
	if cond == nil {
		return currentConditionMet
	}

	switch cond.Type {
	case DurationByCount:
		if currentConditionMet {
			return alert.ConsecutiveHits >= cond.CheckCount
		}
		return false
	case DurationByTime:
		if !currentConditionMet {
			return false
		}
		if alert.FirstFiredTime.IsZero() {
			return false
		}
		return now.Sub(alert.FirstFiredTime) >= cond.TimeWindow
	default:
		return currentConditionMet
	}
}

func checkEscalation(escalations []EscalationRule, alert *AlertState, now time.Time) (bool, AlertLevel) {
	if len(escalations) == 0 {
		return false, alert.CurrentLevel
	}

	if alert.FirstTriggeredTime.IsZero() {
		return false, alert.CurrentLevel
	}

	duration := now.Sub(alert.FirstTriggeredTime)

	for _, esc := range escalations {
		if alert.CurrentLevel == esc.FromLevel && duration >= esc.AfterDuration {
			return true, esc.ToLevel
		}
	}
	return false, alert.CurrentLevel
}

func isInhibitPeriod(rule *AlertRule, alert *AlertState, now time.Time) bool {
	if rule.InhibitDuration <= 0 {
		return false
	}
	if alert.LastNotifiedTime.IsZero() {
		return false
	}
	return now.Sub(alert.LastNotifiedTime) < rule.InhibitDuration
}

func isInSilentPeriod(rule *AlertRule, alert *AlertState, now time.Time) bool {
	for _, sw := range rule.SilentWindows {
		if len(sw.Tags) > 0 {
			ruleTagSet := make(map[string]bool)
			for _, tag := range rule.Tags {
				ruleTagSet[tag] = true
			}
			tagMatched := false
			for _, tag := range sw.Tags {
				if ruleTagSet[tag] {
					tagMatched = true
					break
				}
			}
			if !tagMatched {
				continue
			}
		}

		switch sw.Type {
		case SilentDaily:
			if isInDailySilent(sw, now) {
				return true
			}
		case SilentRange:
			if isInRangeSilent(sw, now) {
				return true
			}
		}
	}
	return false
}

func isInDailySilent(sw SilentWindow, now time.Time) bool {
	startHour, startMin, err := parseTimeStr(sw.StartTime)
	if err != nil {
		return false
	}
	endHour, endMin, err := parseTimeStr(sw.EndTime)
	if err != nil {
		return false
	}

	startTime := time.Date(now.Year(), now.Month(), now.Day(), startHour, startMin, 0, 0, now.Location())
	endTime := time.Date(now.Year(), now.Month(), now.Day(), endHour, endMin, 0, 0, now.Location())

	if endTime.Before(startTime) {
		endTime = endTime.Add(24 * time.Hour)
		nowAdj := now
		if now.Before(startTime) {
			nowAdj = now.Add(24 * time.Hour)
		}
		return !nowAdj.Before(startTime) && nowAdj.Before(endTime)
	}

	return !now.Before(startTime) && now.Before(endTime)
}

func isInRangeSilent(sw SilentWindow, now time.Time) bool {
	return !now.Before(sw.StartDate) && now.Before(sw.EndDate)
}

func parseTimeStr(s string) (int, int, error) {
	var hour, minute int
	n, err := fmt.Sscanf(s, "%d:%d", &hour, &minute)
	if err != nil || n != 2 {
		return 0, 0, ErrInvalidSilentWindow
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, ErrInvalidSilentWindow
	}
	return hour, minute, nil
}

func (e *Engine) sendNotificationsLocked(rule *AlertRule, alert *AlertState, triggerValue float64, now time.Time) {
	notification := Notification{
		RuleID:       rule.ID,
		AlertName:    rule.Name,
		TriggerValue: triggerValue,
		TriggerTime:  now,
		CurrentLevel: alert.CurrentLevel,
		Labels:       rule.Labels,
		Message:      fmt.Sprintf("Alert %s triggered with value %.2f at %s, level: %s", rule.Name, triggerValue, now.Format(time.RFC3339), alert.CurrentLevel),
	}

	notifierNames := e.getNotifiersForLevel(rule, alert.CurrentLevel)
	if len(notifierNames) == 0 {
		notifierNames = make([]string, 0, len(e.notifiers))
		for name := range e.notifiers {
			notifierNames = append(notifierNames, name)
		}
	}

	for _, name := range notifierNames {
		notifier, exists := e.notifiers[name]
		if exists {
			func() {
				defer func() { recover() }()
				_ = notifier.Send(notification)
			}()
		}
	}
}

func (e *Engine) getNotifiersForLevel(rule *AlertRule, level AlertLevel) []string {
	if rule.LevelNotifiers != nil {
		if names, ok := rule.LevelNotifiers[level]; ok && len(names) > 0 {
			return names
		}
	}
	return rule.Notifiers
}

type ConsoleNotifier struct {
	notifications []Notification
	mu            sync.Mutex
}

func NewConsoleNotifier() *ConsoleNotifier {
	return &ConsoleNotifier{
		notifications: make([]Notification, 0),
	}
}

func (c *ConsoleNotifier) Send(notification Notification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications = append(c.notifications, notification)
	fmt.Printf("[ALERT %s] %s - Value: %.2f, Level: %s, Time: %s\n",
		notification.CurrentLevel,
		notification.AlertName,
		notification.TriggerValue,
		notification.CurrentLevel,
		notification.TriggerTime.Format(time.RFC3339))
	return nil
}

func (c *ConsoleNotifier) Name() string {
	return "console"
}

func (c *ConsoleNotifier) Notifications() []Notification {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]Notification, len(c.notifications))
	copy(result, c.notifications)
	return result
}

func (c *ConsoleNotifier) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications = c.notifications[:0]
}

type CallbackNotifier struct {
	name          string
	callback      func(Notification) error
	notifications []Notification
	mu            sync.Mutex
}

func NewCallbackNotifier(name string, callback func(Notification) error) *CallbackNotifier {
	return &CallbackNotifier{
		name:          name,
		callback:      callback,
		notifications: make([]Notification, 0),
	}
}

func (c *CallbackNotifier) Send(notification Notification) error {
	c.mu.Lock()
	c.notifications = append(c.notifications, notification)
	callback := c.callback
	c.mu.Unlock()

	if callback != nil {
		return callback(notification)
	}
	return nil
}

func (c *CallbackNotifier) Name() string {
	return c.name
}

func (c *CallbackNotifier) Notifications() []Notification {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]Notification, len(c.notifications))
	copy(result, c.notifications)
	return result
}

func (c *CallbackNotifier) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications = c.notifications[:0]
}
