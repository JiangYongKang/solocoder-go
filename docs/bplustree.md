# B+Tree 索引结构模块

## 1. 模块概述

B+Tree 是一种平衡多路搜索树索引结构，广泛用于数据库和文件系统的索引实现。本模块实现了完整的 B+ 树操作，包括键值插入、节点自动分裂、范围扫描和基于游标的迭代器遍历。

**包路径**: `internal/bplustree`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 键值插入 | 支持向 B+ 树中插入键值对，数据仅存储在叶子节点，内部节点仅存储索引键和子节点指针 |
| 节点分裂 | 当节点键数量超过配置上限时自动分裂，中间键提升到父节点，支持连锁向上分裂至根节点 |
| 键值删除 | 支持按键删除键值对，删除后自动处理节点下溢（借键或合并）以维持树的平衡 |
| 节点下溢处理 | 叶子/内部节点键数低于阈值时，优先从左/右兄弟借键，否则与兄弟合并，必要时连锁向上收缩树结构 |
| 范围扫描 | 按起始键和结束键进行范围查询，沿叶子链表顺序遍历返回范围内的所有键值对 |
| 游标迭代器 | 支持前向（Next）和后向（Prev）遍历，支持在删除当前元素后继续遍历，删除逻辑复用树的 Delete 方法 |

## 3. 核心结构体与职责

### 3.1 BPlusTree

B+ 树的主结构体，对外提供所有操作接口。

```go
type BPlusTree struct {
    root    *node
    maxKeys int
    count   int
}
```

**职责**:
- 维护树的根节点和全局配置
- 提供 Insert、Search、Delete、RangeScan 等操作入口
- 管理节点分裂的递归传播与节点下溢的借键/合并/收缩
- 跟踪树中键值对的总数
- 计算节点下溢阈值：`minKeys() = (maxKeys + 1) / 2`

### 3.2 node

B+ 树的节点，同时表示内部节点和叶子节点。

```go
type node struct {
    keys     []string
    isLeaf   bool
    parent   *node
    next     *node
    prev     *node
    children []*node
    values   []string
}
```

**职责**:
- **叶子节点** (`isLeaf=true`): `keys` 存储排序后的键，`values` 存储对应的值，`next`/`prev` 构成叶子链表
- **内部节点** (`isLeaf=false`): `keys` 存储路由索引键，`children` 存储子节点指针，`children[i]` 中的键均小于 `keys[i]`，`children[i+1]` 中的键均大于等于 `keys[i]`
- `parent` 指向父节点，用于分裂时向上传播

### 3.3 Iterator

基于游标的迭代器，支持在 B+ 树上顺序遍历。

```go
type Iterator struct {
    tree   *BPlusTree
    node   *node
    index  int
    key    string
    valid  bool
}
```

**职责**:
- 维护当前遍历位置（节点 + 节点内索引 + 当前键身份校验值）
- 支持 Next/Prev 前向后向移动
- 支持 Delete 删除当前元素，通过 `key` 字段进行三重身份校验防止静默漂移
- 删除后自动定位到下一个有效元素

### 3.4 Config 与 KVItem

```go
type Config struct {
    MaxKeys int
}

type KVItem struct {
    Key   string
    Value string
}
```

- **Config**: 配置 B+ 树的最大键数量（每个节点），最小值为 2，奇数值会自动向上取偶
- **KVItem**: 范围扫描返回的键值对结果

## 4. 节点分裂流程

### 4.1 叶子节点分裂

1. 向叶子节点插入键值对后，若 `len(keys) > maxKeys`，触发分裂
2. 取中间位置 `mid = len(keys) / 2`
3. 将 `keys[mid:]` 和 `values[mid:]` 拆分到新的右节点
4. 左节点保留 `keys[:mid]` 和 `values[:mid]`
5. 更新叶子链表指针：右节点的 `next` 指向原左节点的 `next`，左节点的 `next` 指向右节点，原后继节点的 `prev` 指向右节点
6. 将右节点的第一个键作为索引键提升到父节点

### 4.2 内部节点分裂

1. 向内部节点插入索引键和子节点后，若 `len(keys) > maxKeys`，触发分裂
2. 取中间位置 `mid = len(keys) / 2`，中间键为 `keys[mid]`
3. 将 `keys[mid+1:]` 拆分到新的右节点，`children[mid+1:]` 拆分到右节点
4. 左节点保留 `keys[:mid]` 和 `children[:mid+1]`
5. 中间键 `keys[mid]` 提升到父节点（不再保留在子节点中）
6. 更新右节点所有子节点的 `parent` 指针

### 4.3 连锁分裂与根分裂

- 分裂后向父节点插入索引键，若父节点也溢出，则继续分裂父节点
- 当根节点分裂时，创建新的根节点，树的高度增加 1
- 分裂过程递归向上传播，直到某层节点不再溢出或到达新根

### 4.4 分裂示例

以 `MaxKeys=4` 为例，依次插入 a, b, c, d, e：

```
插入 a, b, c, d → 叶子节点: [a, b, c, d] (4个键，等于 maxKeys，不分裂)
插入 e → 叶子节点溢出: [a, b, c, d, e] (5个键)
分裂:
  左叶子: [a, b]      右叶子: [c, d, e]
  新根(内部节点): [c]  → children: [左叶子, 右叶子]
```

## 5. 节点下溢处理流程

节点下溢指删除后节点键数量低于最小阈值 `minKeys = (maxKeys + 1) / 2`。B+ 树通过借键或合并来维持自平衡。

### 5.1 叶子节点下溢

#### 借键（优先）

当兄弟节点键数量充足（`> minKeys`）时，从兄弟借一个键：

**从左兄弟借** (`borrowFromLeftLeaf`):
1. 取左兄弟最后一个键值对 `(last_key, last_value)`
2. 从左兄弟移除该键值对
3. 将该键值对插入当前节点的首部
4. 更新父节点中分隔左兄弟与当前节点的索引键为当前节点的新首键

**从右兄弟借** (`borrowFromRightLeaf`):
1. 取右兄弟第一个键值对 `(first_key, first_value)`
2. 从右兄弟移除该键值对
3. 将该键值对追加到当前节点的尾部
4. 更新父节点中分隔当前节点与右兄弟的索引键为右兄弟的新首键

#### 合并（兄弟也无法借时）

当兄弟节点键数量刚好等于 `minKeys` 时，合并两个节点：

**与左兄弟合并** (`mergeWithLeftLeaf`):
1. 将当前节点的所有键值追加到左兄弟的尾部
2. 更新叶子链表：左兄弟的 `next` 指向当前节点的 `next`，若后继存在则其 `prev` 指向左兄弟
3. 从父节点删除分隔左兄弟与当前节点的索引键及当前节点子节点指针
4. 父节点删除后若下溢则递归处理父节点

**与右兄弟合并** (`mergeWithRightLeaf`):
1. 将右兄弟的所有键值追加到当前节点的尾部
2. 更新叶子链表：当前节点的 `next` 指向右兄弟的 `next`，若后继存在则其 `prev` 指向当前节点
3. 从父节点删除分隔当前节点与右兄弟的索引键及右兄弟子节点指针
4. 父节点删除后若下溢则递归处理父节点

### 5.2 内部节点下溢

#### 借键（优先）

**从左兄弟借** (`borrowFromLeftInternal`):
1. 取父节点中的分隔键 `separator`（分隔左兄弟与当前节点）
2. 取左兄弟的最后一个键 `moved_key` 和最后一个子节点 `moved_child`
3. 从左兄弟移除 `moved_key` 和 `moved_child`
4. 将 `separator` 插入当前节点 keys 首部，`moved_child` 插入当前节点 children 首部并更新其 parent
5. 将父节点中的分隔键更新为 `moved_key`

**从右兄弟借** (`borrowFromRightInternal`):
1. 取父节点中的分隔键 `separator`（分隔当前节点与右兄弟）
2. 取右兄弟的第一个键 `moved_key` 和第一个子节点 `moved_child`
3. 从右兄弟移除 `moved_key` 和 `moved_child`
4. 将 `separator` 追加到当前节点 keys 尾部，`moved_child` 追加到当前节点 children 尾部并更新其 parent
5. 将父节点中的分隔键更新为 `moved_key`

#### 合并（兄弟也无法借时）

**与左兄弟合并** (`mergeWithLeftInternal`):
1. 取父节点中的分隔键 `separator`
2. 将 `separator` 追加到左兄弟 keys 尾部，再追加当前节点的所有 keys
3. 将当前节点的所有 children 追加到左兄弟 children 尾部，并更新这些子节点的 parent 指向左兄弟
4. 从父节点删除分隔键及当前节点子节点指针
5. 父节点删除后若下溢则递归处理

**与右兄弟合并** (`mergeWithRightInternal`):
1. 取父节点中的分隔键 `separator`
2. 将 `separator` 追加到当前节点 keys 尾部，再追加右兄弟的所有 keys
3. 将右兄弟的所有 children 追加到当前节点 children 尾部，并更新这些子节点的 parent 指向当前节点
4. 从父节点删除分隔键及右兄弟子节点指针
5. 父节点删除后若下溢则递归处理

### 5.3 根节点收缩

当根节点为内部节点且删除后 `keys` 为空、仅剩余 1 个子节点时：
- 将根节点替换为该唯一子节点
- 清空新根的 parent 指针
- 树的高度减少 1

### 5.4 删除后树结构收缩保证

删除操作的完整流程确保了 B+ 树始终满足自平衡约束：
1. 每个非根节点的键数 ∈ `[minKeys, maxKeys]`
2. 根节点若为内部节点，至少有 2 个子节点
3. 所有叶子节点位于同一层
4. 叶子链表保持完整的双向连接，支持高效的范围扫描
5. 父节点的分隔键始终正确反映子节点的键范围边界

### 5.5 下溢处理示例

以 `MaxKeys=4`（即 `minKeys=2`）为例，初始树结构：
```
内部节点: [d]
  叶子1: [a, b, c]
  叶子2: [d, e, f]
```

**场景1：删除后触发借键**

删除 `e`：叶子2变为 `[d, f]`（2个键，等于minKeys，不下溢）
删除 `f`：叶子2变为 `[d]`（1个键 < minKeys，下溢）
→ 左兄弟叶子1有3个键（> minKeys），从左兄弟借键
→ 借 `c`：叶子1变为 `[a, b]`，叶子2变为 `[c, d]`
→ 更新父节点分隔键为 `c`

最终：
```
内部节点: [c]
  叶子1: [a, b]
  叶子2: [c, d]
```

**场景2：删除后触发合并与根收缩**

继续删除 `d`：叶子2变为 `[c]`（下溢）
→ 左兄弟叶子1有2个键（= minKeys，无法借），与左兄弟合并
→ 合并后叶子变为 `[a, b, c]`
→ 从父节点删除分隔键 `c`，父节点 `keys` 变空，仅余1个子节点
→ 根节点收缩为该子节点

最终：
```
叶子(根): [a, b, c]
```

### 5.6 Iterator.Delete 与 tree.Delete 的协作关系

Iterator.Delete 完全依赖 tree.Delete 完成实际的删除操作和节点下溢平衡，二者的协作流程如下：

```
Iterator.Delete()
     │
     ▼
1.  有效性与身份校验
    ├─ 迭代器 invalid → 返回 ErrIteratorInvalid
    ├─ index 越界 → 返回 ErrKeyNotFound
    └─ 键身份不匹配（静默漂移）→ 返回 ErrKeyNotFound
     │
     ▼
2.  采集重定位锚点（复用已有方法）
    ├─ prevKey：通过 Prev() 方法获取后恢复状态
    └─ nextKey：删除后通过 NewIteratorAt(deletedKey) 自动定位
     │
     ▼
3.  调用 tree.Delete(key) ────┐
     │                         │
     │                         ▼
     │                    tree.Delete 内部完整执行：
     │                      ├─ findLeaf 定位叶子节点
     │                      ├─ 删除键值对
     │                      ├─ 叶子节点下溢处理（借键/合并）
     │                      ├─ 内部节点下溢处理（借键/合并）
     │                      └─ 根节点收缩
     │                         │
     │                         ▼
     │                    返回 bool（是否成功删除）
     │                         │
     └─────────────────────────┘
     │
     ▼
4.  tree.Delete 返回 false → 返回 ErrKeyNotFound
     │
     ▼
5.  复用 NewIteratorAt 重定位
    ├─ 用被删除键调用 NewIteratorAt → 自动定位到下一个元素
    ├─ 失败则用 prevKey 调用 NewIteratorAt 回退定位
    └─ 都失败则标记迭代器为 invalid
```

**核心设计原则：单一职责与代码复用**

- **tree.Delete 是唯一的删除入口**：所有键值删除、节点下溢检测、借键、合并、根收缩等逻辑仅在 tree.Delete 中实现和维护。
- **Iterator.Delete 不重复实现删除逻辑**：只负责：(1) 验证迭代器当前位置的有效性和键身份；(2) 通过已有方法采集重定位锚点；(3) 委托 tree.Delete 执行实际删除；(4) 复用 NewIteratorAt 完成删除后的重定位。
- **下溢处理对 Iterator 透明**：tree.Delete 内部的节点借键、合并、根收缩等平衡操作完全不影响 Iterator.Delete 的逻辑，迭代器只需在删除完成后通过锚点键重新定位即可。

**键身份校验与静默漂移防护**

Iterator 结构体维护一个 `key` 字段缓存当前键（每次 Next/Prev/NewIteratorAt 时同步更新），Delete 时执行三重校验：

1. **valid 状态校验**：迭代器是否处于有效状态
2. **index 范围校验**：index 是否在节点 keys 数组的合法范围内
3. **键身份校验**：`node.keys[index] == key`，确保迭代器指向的键仍是最初定位的键

**静默漂移场景**：当外部通过 tree.Delete 删除迭代器当前节点中位于 index 之前的键时，keys 数组会缩短，但 index 仍在合法范围内，此时迭代器指向的键已悄然变化（index 不变但键右移）。键身份校验能够检测到这种静默漂移，返回 ErrKeyNotFound 而非错误地删除了另一个键。

**ErrKeyNotFound 的三种可达场景**：

1.  **index 越界**：外部删除导致 keys 数组缩短，index 超出新的数组边界。
2.  **静默漂移（键身份不匹配）**：index 仍在合法范围内，但指向的键已不是最初定位的键。
3.  **键已被外部删除**：tree.Delete 在内部查找键时发现键不存在，返回 false，Iterator.Delete 将其转换为 ErrKeyNotFound。

**重定位锚点采集的复用策略**

重定位锚点的采集完全复用已有方法，不独立实现遍历逻辑：

- **prevKey 采集**：通过调用 `Prev()` 方法移动到前一个元素获取键值，然后恢复迭代器原有状态。完全复用 Prev() 的跨节点跳转逻辑。
- **nextKey 采集**：删除完成后，直接用被删除的键调用 `NewIteratorAt(deletedKey)`。由于该键已不存在，NewIteratorAt 会自动定位到第一个大于该键的元素，也就是删除位置的下一个元素。完全复用 NewIteratorAt 的 findLeaf + 线性扫描逻辑。

**删除后重定位策略**：

删除操作可能导致节点合并、借键、根收缩等结构变化，原有的 node/index 指针可能完全失效。因此 Iterator.Delete 采用"锚点键 + NewIteratorAt"的重定位策略：

- 优先用被删除键调用 NewIteratorAt 重新定位（自动指向下一个元素），确保遍历可以继续前进
- 如果下一个元素不存在（删除的是最后一个元素），则用 prevKey 回退定位
- 完全复用 NewIteratorAt 的 findLeaf + 线性扫描定位逻辑，确保重定位的正确性与代码一致性

## 6. API 参考

### 6.1 构造函数

```go
func NewBPlusTree() *BPlusTree
func NewBPlusTreeWithConfig(cfg Config) *BPlusTree
func DefaultConfig() Config
```

### 6.2 基本操作

```go
func (t *BPlusTree) Insert(key string, value string)
func (t *BPlusTree) Search(key string) (string, bool)
func (t *BPlusTree) Delete(key string) bool
func (t *BPlusTree) Count() int
```

### 6.3 范围扫描

```go
func (t *BPlusTree) RangeScan(start, end string) ([]KVItem, error)
```

### 6.4 迭代器

```go
func (t *BPlusTree) NewIterator() *Iterator
func (t *BPlusTree) NewIteratorAt(key string) *Iterator
func (it *Iterator) Valid() bool
func (it *Iterator) Key() (string, error)
func (it *Iterator) Value() (string, error)
func (it *Iterator) Next() error
func (it *Iterator) Prev() error
func (it *Iterator) Delete() error
```

## 7. 使用示例

### 7.1 基本插入与查询

```go
tree := bplustree.NewBPlusTree()

tree.Insert("user:1", "Alice")
tree.Insert("user:2", "Bob")
tree.Insert("user:3", "Charlie")

val, ok := tree.Search("user:2")
// val = "Bob", ok = true

val, ok = tree.Search("user:99")
// val = "", ok = false
```

### 7.2 使用自定义配置

```go
cfg := bplustree.Config{MaxKeys: 8}
tree := bplustree.NewBPlusTreeWithConfig(cfg)
```

### 7.3 范围扫描

```go
tree := bplustree.NewBPlusTree()
tree.Insert("apple", "1")
tree.Insert("banana", "2")
tree.Insert("cherry", "3")
tree.Insert("date", "4")

items, err := tree.RangeScan("banana", "cherry")
// items = []KVItem{{Key:"banana", Value:"2"}, {Key:"cherry", Value:"3"}}
```

### 7.4 迭代器前向遍历

```go
tree := bplustree.NewBPlusTree()
tree.Insert("a", "1")
tree.Insert("b", "2")
tree.Insert("c", "3")

it := tree.NewIterator()
for it.Valid() {
    key, _ := it.Key()
    val, _ := it.Value()
    fmt.Printf("%s = %s\n", key, val)
    it.Next()
}
// 输出: a = 1, b = 2, c = 3
```

### 7.5 迭代器定位与后向遍历

```go
it := tree.NewIteratorAt("b")
for it.Valid() {
    key, _ := it.Key()
    fmt.Println(key)
    it.Prev()
}
// 输出: b, a
```

### 7.6 迭代器删除当前元素后继续遍历

```go
it := tree.NewIterator()
for it.Valid() {
    key, _ := it.Key()
    if key == "b" {
        it.Delete()  // 删除 "b"，迭代器自动定位到下一个元素
        continue
    }
    fmt.Println(key)
    it.Next()
}
// 输出: a, c
```

### 7.7 键值删除

```go
tree.Delete("user:1")
_, ok := tree.Search("user:1")
// ok = false
```

## 8. 错误定义

| 错误 | 触发场景 |
|------|----------|
| `ErrKeyNotFound` | Iterator.Delete 时 index 越界、键身份不匹配(静默漂移)或键已被外部删除 |
| `ErrInvalidRange` | RangeScan 的 start > end |
| `ErrInvalidMaxKeys` | MaxKeys 配置无效（< 2） |
| `ErrIteratorInvalid` | 在无效迭代器上调用 Key/Value/Next/Prev/Delete |
| `ErrIteratorDone` | 迭代器已遍历到边界（首元素之前或末元素之后） |
