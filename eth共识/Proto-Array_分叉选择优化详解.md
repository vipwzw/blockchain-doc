# Proto-Array 分叉选择优化详解

> 从以太坊 PoS 到比特币最长链：高效分叉选择的数据结构与算法

---

## 摘要

Proto-Array 是以太坊 PoS 客户端中用于优化 LMD-GHOST 分叉选择的核心数据结构。本文深入解析 Proto-Array 的设计原理、数据结构、算法实现，并探讨如何将这一优化思想迁移到比特币的最长链选择机制中，实现高效的分叉管理。

**关键词**：Proto-Array、分叉选择、LMD-GHOST、最长链、区块链优化

---

## 目录

1. [问题背景](#1-问题背景)
2. [Proto-Array 核心设计](#2-proto-array-核心设计)
3. [数据结构详解](#3-数据结构详解)
4. [核心算法](#4-核心算法)
5. [完整实现示例](#5-完整实现示例)
6. [迁移到比特币最长链](#6-迁移到比特币最长链)
7. [Bitcoin Core 实际实现分析](#7-bitcoin-core-实际实现分析)
8. [性能分析与对比](#8-性能分析与对比)
9. [总结](#9-总结)

---

## 1. 问题背景

### 1.1 朴素分叉选择的问题

在区块链系统中，分叉选择是核心操作之一。每当收到新区块或新投票时，节点需要确定当前的规范链头。

**以太坊 LMD-GHOST 的朴素实现**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      朴素 LMD-GHOST 算法                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   每次分叉选择：                                                        │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   1. 从 justified checkpoint 开始                               │  │
│   │   2. 遍历所有子节点                                              │  │
│   │   3. 对每个子节点，遍历所有验证者投票计算权重                    │  │
│   │   4. 选择权重最大的子节点                                        │  │
│   │   5. 递归直到叶子节点                                            │  │
│   │                                                                  │  │
│   │   复杂度：O(nodes × validators) per fork choice                 │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   问题：                                                                │
│   • 100万验证者 × 每秒数千条证明 = 计算灾难                            │
│   • 每次重新遍历整棵树                                                 │
│   • 哈希表查找开销大                                                   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**比特币最长链的朴素实现**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      朴素最长链选择                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   每次收到新区块：                                                      │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   1. 验证区块有效性                                              │  │
│   │   2. 找到父区块                                                  │  │
│   │   3. 计算该链的总工作量/高度                                     │  │
│   │   4. 与当前最长链比较                                            │  │
│   │   5. 如果更长，切换到新链                                        │  │
│   │                                                                  │  │
│   │   问题：                                                         │  │
│   │   • 需要维护所有分叉分支                                         │  │
│   │   • 链切换时需要回溯找公共祖先                                   │  │
│   │   • 哈希表查找父区块                                             │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 1.2 设计目标

Proto-Array 的设计目标：

| 目标 | 要求 |
|-----|------|
| **分叉选择** | O(1) 或 O(depth) 时间复杂度 |
| **新区块处理** | O(depth) 时间复杂度 |
| **权重更新** | 增量更新，O(depth) |
| **内存占用** | 只维护未确定的区块 |
| **访问效率** | 数组索引替代哈希查找 |

---

## 2. Proto-Array 核心设计

### 2.1 核心思想

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Proto-Array 三大核心思想                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   思想 1：树展平为数组                                                  │
│   ═══════════════════════                                              │
│                                                                         │
│   树结构：                         数组结构：                           │
│       A                            [A, B, C, D, E, F]                   │
│      / \                            0  1  2  3  4  5                    │
│     B   C                                                               │
│    / \   \                          parent 指针用索引表示：             │
│   D   E   F                         A.parent = None                     │
│                                     B.parent = 0 (A)                    │
│                                     C.parent = 0 (A)                    │
│                                     D.parent = 1 (B)                    │
│                                     ...                                 │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   思想 2：缓存最佳后代                                                  │
│   ═══════════════════                                                  │
│                                                                         │
│   每个节点维护：                                                        │
│   • best_child: 权重最大的直接子节点                                   │
│   • best_descendant: 该子树中的最佳叶子节点（链头候选）                │
│                                                                         │
│       A ──────────────────────────────┐                                │
│      / \                              │ best_descendant                │
│     B   C                             ▼                                │
│    / \   \                           [E]                               │
│   D   E   F                                                            │
│       ▲                                                                 │
│       └── B.best_child = E                                             │
│           B.best_descendant = E                                        │
│           A.best_child = B                                             │
│           A.best_descendant = E                                        │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   思想 3：只维护 Finalized 之后                                         │
│   ════════════════════════════                                         │
│                                                                         │
│   [Genesis] ← ... ← [Finalized] ← [Block] ← [Block] ← [Head]          │
│   ══════════════════════════════  ════════════════════════════         │
│         已确定，丢弃                    Proto-Array 维护               │
│                                                                         │
│   Finalized 推进时，剪枝旧节点                                         │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 工作流程概览

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Proto-Array 工作流程                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                         事件触发                                 │  │
│   └───────────────────────────┬─────────────────────────────────────┘  │
│                               │                                         │
│           ┌───────────────────┼───────────────────┐                    │
│           ▼                   ▼                   ▼                    │
│   ┌───────────────┐   ┌───────────────┐   ┌───────────────┐           │
│   │   新区块到达   │   │   新投票到达   │   │ Finalized更新 │           │
│   └───────┬───────┘   └───────┬───────┘   └───────┬───────┘           │
│           │                   │                   │                    │
│           ▼                   ▼                   ▼                    │
│   ┌───────────────┐   ┌───────────────┐   ┌───────────────┐           │
│   │ 添加新节点    │   │ 增量更新权重   │   │   剪枝旧节点   │           │
│   │ 到数组末尾    │   │ 向上传播       │   │   更新索引     │           │
│   └───────┬───────┘   └───────┬───────┘   └───────────────┘           │
│           │                   │                                        │
│           ▼                   ▼                                        │
│   ┌───────────────────────────────────────┐                           │
│   │         更新 best_child/best_descendant │                           │
│   └───────────────────────────────────────┘                           │
│                               │                                         │
│                               ▼                                         │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                    分叉选择：O(1)                                │  │
│   │         直接返回根节点的 best_descendant                         │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 3. 数据结构详解

### 3.1 ProtoNode 结构

```rust
/// 单个区块节点
struct ProtoNode {
    /// 区块哈希（用于外部查询）
    root: Hash,
    
    /// 父节点在数组中的索引（None 表示根节点）
    parent: Option<usize>,
    
    /// 区块所在的 slot（用于比较）
    slot: u64,
    
    /// 该节点的累积权重（包含所有后代的投票）
    weight: u64,
    
    /// 权重最大的直接子节点索引
    best_child: Option<usize>,
    
    /// 该子树中最佳的叶子节点索引（分叉选择结果）
    best_descendant: Option<usize>,
    
    /// justified 和 finalized 状态（用于验证）
    justified_epoch: u64,
    finalized_epoch: u64,
}
```

**字段可视化**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      ProtoNode 字段说明                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   节点 B 的各字段：                                                     │
│                                                                         │
│           A (index=0)                                                   │
│          /│\                                                            │
│         / │ \                                                           │
│        B  C  G                                                          │
│       /\  │                                                             │
│      D  E F         假设 E 权重最大                                     │
│                                                                         │
│   B = ProtoNode {                                                       │
│       root: 0xB...,                                                     │
│       parent: Some(0),      // 指向 A                                  │
│       slot: 100,                                                        │
│       weight: 150,          // D(30) + E(80) + B自身(40) = 150         │
│       best_child: Some(4),  // 指向 E（假设 E 是 index 4）             │
│       best_descendant: Some(4), // E 也是最佳后代（叶子）              │
│       ...                                                               │
│   }                                                                     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.2 ProtoArray 结构

```rust
/// Proto-Array 主结构
struct ProtoArray {
    /// 所有节点（仅 finalized 之后的）
    nodes: Vec<ProtoNode>,
    
    /// 区块哈希 → 数组索引的映射
    indices: HashMap<Hash, usize>,
    
    /// 当前 justified checkpoint
    justified_checkpoint: Checkpoint,
    
    /// 当前 finalized checkpoint（数组的逻辑根）
    finalized_checkpoint: Checkpoint,
}
```

### 3.3 内存布局

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Proto-Array 内存布局                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   nodes 数组：                                                          │
│   ┌─────┬─────┬─────┬─────┬─────┬─────┬─────┬─────┐                    │
│   │  0  │  1  │  2  │  3  │  4  │  5  │  6  │ ... │                    │
│   │ Fin │  A  │  B  │  C  │  D  │  E  │  F  │     │                    │
│   └──┬──┴──┬──┴──┬──┴──┬──┴──┬──┴──┬──┴──┬──┴─────┘                    │
│      │     │     │     │     │     │                                   │
│      │     │     │     └──┬──┘     │                                   │
│      │     │     │        │        │                                   │
│      │     └─────┼────────┼────────┘                                   │
│      │           │        │                                            │
│      └───────────┴────────┘                                            │
│           parent 索引指向                                               │
│                                                                         │
│   indices 映射：                                                        │
│   ┌───────────────────────────────────────┐                            │
│   │  0xFin... → 0                         │                            │
│   │  0xA...   → 1                         │                            │
│   │  0xB...   → 2                         │                            │
│   │  ...                                  │                            │
│   └───────────────────────────────────────┘                            │
│                                                                         │
│   连续内存访问，CPU 缓存友好                                           │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 4. 核心算法

### 4.1 添加新区块

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      on_block: 添加新区块                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   输入：新区块 Block                                                    │
│   输出：更新后的 Proto-Array                                            │
│                                                                         │
│   流程：                                                                │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   1. 创建新 ProtoNode                                            │  │
│   │      node = ProtoNode {                                          │  │
│   │          root: block.hash,                                       │  │
│   │          parent: indices[block.parent_hash],                     │  │
│   │          weight: 0,  // 初始无投票                               │  │
│   │          best_child: None,                                       │  │
│   │          best_descendant: None,  // 自己就是最佳后代             │  │
│   │      }                                                           │  │
│   │                                                                  │  │
│   │   2. 追加到数组末尾                                              │  │
│   │      new_index = nodes.len()                                     │  │
│   │      nodes.push(node)                                            │  │
│   │      indices[block.hash] = new_index                             │  │
│   │                                                                  │  │
│   │   3. 设置自己为最佳后代                                          │  │
│   │      nodes[new_index].best_descendant = Some(new_index)          │  │
│   │                                                                  │  │
│   │   4. 向上更新 best_child/best_descendant                         │  │
│   │      maybe_update_best_child_and_descendant(parent_index)        │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

```python
def on_block(proto_array, block):
    """添加新区块到 Proto-Array"""
    
    # 查找父节点索引
    parent_index = proto_array.indices.get(block.parent_root)
    if parent_index is None:
        raise Exception("Parent block not found")
    
    # 创建新节点
    new_index = len(proto_array.nodes)
    node = ProtoNode(
        root=block.root,
        parent=parent_index,
        slot=block.slot,
        weight=0,
        best_child=None,
        best_descendant=new_index,  # 叶子节点的最佳后代是自己
        justified_epoch=block.justified_epoch,
        finalized_epoch=block.finalized_epoch,
    )
    
    # 添加到数组和索引
    proto_array.nodes.append(node)
    proto_array.indices[block.root] = new_index
    
    # 向上更新父节点的 best_child 和 best_descendant
    update_ancestor_best(proto_array, parent_index, new_index)
```

### 4.2 处理投票（权重更新）

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      on_attestation: 处理投票                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   关键优化：增量更新，只处理变化部分                                    │
│                                                                         │
│   场景：验证者 V 从投票 Block A 改为投票 Block B                        │
│                                                                         │
│           Root                                                          │
│          / | \                                                          │
│         X  Y  Z                                                         │
│        /  / \                                                           │
│       A  B   C        V 的投票: A → B                                  │
│                                                                         │
│   操作：                                                                │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   1. 从 A 向上减少权重                                           │  │
│   │      A.weight -= V.balance                                       │  │
│   │      X.weight -= V.balance                                       │  │
│   │      Root.weight -= V.balance                                    │  │
│   │                                                                  │  │
│   │   2. 从 B 向上增加权重                                           │  │
│   │      B.weight += V.balance                                       │  │
│   │      Y.weight += V.balance                                       │  │
│   │      Root.weight += V.balance                                    │  │
│   │                                                                  │  │
│   │   3. 沿途更新 best_child/best_descendant                         │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   注意：如果 A 和 B 有公共祖先 Root，Root 的权重实际不变              │
│   但需要更新其 best_child（X → Y 可能发生变化）                       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

```python
def on_attestation(proto_array, validator_index, new_target, balance):
    """处理验证者投票变化"""
    
    # 获取验证者之前的投票
    old_target = latest_messages.get(validator_index)
    
    if old_target == new_target:
        return  # 投票未变，无需更新
    
    # 从旧目标向上减少权重
    if old_target is not None:
        apply_weight_delta(proto_array, old_target, -balance)
    
    # 向新目标向上增加权重
    apply_weight_delta(proto_array, new_target, +balance)
    
    # 更新最新消息
    latest_messages[validator_index] = new_target

def apply_weight_delta(proto_array, block_root, delta):
    """沿路径向上传播权重变化"""
    
    node_index = proto_array.indices.get(block_root)
    if node_index is None:
        return
    
    # 沿 parent 链向上遍历到根
    while node_index is not None:
        node = proto_array.nodes[node_index]
        
        # 更新权重
        node.weight += delta
        
        # 更新父节点的 best_child（如果有父节点）
        parent_index = node.parent
        if parent_index is not None:
            update_best_child_and_descendant(proto_array, parent_index)
        
        node_index = parent_index
```

### 4.3 更新 best_child 和 best_descendant

这是 Proto-Array 的核心逻辑：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      update_best_child_and_descendant                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   对于节点 P，需要确定：                                                │
│   • best_child: P 的所有子节点中权重最大的                             │
│   • best_descendant: best_child 子树中的最佳叶子                       │
│                                                                         │
│   关键：不需要遍历所有子节点！                                          │
│                                                                         │
│   原因：权重变化只影响一条路径，最多只需比较：                          │
│   • 当前 best_child                                                     │
│   • 权重刚变化的那个子节点                                              │
│                                                                         │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   情况分析：                                                     │  │
│   │                                                                  │  │
│   │   设 P 当前的 best_child = X，刚更新权重的子节点 = Y            │  │
│   │                                                                  │  │
│   │   if Y.weight > X.weight:                                        │  │
│   │       P.best_child = Y                                           │  │
│   │       P.best_descendant = Y.best_descendant                      │  │
│   │   else:                                                          │  │
│   │       # best_child 不变，但 X 的 best_descendant 可能变了        │  │
│   │       P.best_descendant = X.best_descendant                      │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

```python
def update_best_child_and_descendant(proto_array, parent_index):
    """更新节点的 best_child 和 best_descendant"""
    
    parent = proto_array.nodes[parent_index]
    
    # 获取所有子节点（通过遍历找 parent 指向自己的节点）
    # 优化：实际实现中会维护 children 列表
    children = get_children(proto_array, parent_index)
    
    if not children:
        parent.best_child = None
        parent.best_descendant = parent_index  # 自己是叶子
        return
    
    # 找权重最大的子节点
    best_child_index = max(
        children,
        key=lambda idx: (
            proto_array.nodes[idx].weight,
            proto_array.nodes[idx].root  # 权重相同时按哈希排序
        )
    )
    
    parent.best_child = best_child_index
    
    # best_descendant 继承自 best_child
    best_child = proto_array.nodes[best_child_index]
    parent.best_descendant = best_child.best_descendant
```

### 4.4 分叉选择（O(1)）

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      get_head: 分叉选择                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   有了 best_descendant 缓存，分叉选择变成 O(1)：                        │
│                                                                         │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   def get_head(proto_array):                                     │  │
│   │       justified_index = proto_array.indices[                     │  │
│   │           proto_array.justified_checkpoint.root                  │  │
│   │       ]                                                          │  │
│   │       justified_node = proto_array.nodes[justified_index]        │  │
│   │                                                                  │  │
│   │       # 直接获取预计算的最佳后代                                 │  │
│   │       best_index = justified_node.best_descendant                │  │
│   │       return proto_array.nodes[best_index].root                  │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   为什么是 O(1)？                                                       │
│   • 一次索引查找获取 justified 节点                                    │
│   • 一次数组访问获取 best_descendant                                   │
│   • 一次数组访问获取链头哈希                                           │
│                                                                         │
│   无需任何遍历！                                                        │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 4.5 剪枝（Pruning）

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      prune: 剪枝旧节点                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   当 finalized checkpoint 推进时，移除旧节点：                          │
│                                                                         │
│   Before:                                                               │
│   ┌─────┬─────┬─────┬─────┬─────┬─────┬─────┬─────┐                    │
│   │  0  │  1  │  2  │  3  │  4  │  5  │  6  │  7  │                    │
│   │F_old│  A  │  B  │F_new│  C  │  D  │  E  │  F  │                    │
│   └─────┴─────┴─────┴─────┴─────┴─────┴─────┴─────┘                    │
│                        ▲                                                │
│                     新 finalized                                        │
│                                                                         │
│   After (移除 index 0-2):                                               │
│   ┌─────┬─────┬─────┬─────┬─────┐                                      │
│   │  0  │  1  │  2  │  3  │  4  │                                      │
│   │F_new│  C  │  D  │  E  │  F  │                                      │
│   └─────┴─────┴─────┴─────┴─────┘                                      │
│                                                                         │
│   需要更新：                                                            │
│   • 所有 parent 索引减去偏移量 (3)                                     │
│   • 所有 best_child 索引减去偏移量                                     │
│   • 所有 best_descendant 索引减去偏移量                                │
│   • 重建 indices 映射                                                  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

```python
def prune(proto_array, new_finalized_root):
    """剪枝 finalized 之前的节点"""
    
    new_finalized_index = proto_array.indices[new_finalized_root]
    
    if new_finalized_index == 0:
        return  # 无需剪枝
    
    # 计算偏移量
    offset = new_finalized_index
    
    # 移除旧节点
    proto_array.nodes = proto_array.nodes[offset:]
    
    # 更新所有索引
    for node in proto_array.nodes:
        if node.parent is not None:
            if node.parent < offset:
                node.parent = None  # 父节点被剪掉了
            else:
                node.parent -= offset
        
        if node.best_child is not None:
            node.best_child -= offset
        
        if node.best_descendant is not None:
            node.best_descendant -= offset
    
    # 重建哈希→索引映射
    proto_array.indices = {
        node.root: i for i, node in enumerate(proto_array.nodes)
    }
    
    proto_array.finalized_checkpoint = Checkpoint(
        epoch=proto_array.nodes[0].finalized_epoch,
        root=new_finalized_root
    )
```

---

## 5. 完整实现示例

### 5.1 Python 完整实现

```python
from dataclasses import dataclass, field
from typing import Optional, Dict, List
from hashlib import sha256

@dataclass
class ProtoNode:
    """Proto-Array 节点"""
    root: bytes
    parent: Optional[int]
    slot: int
    weight: int = 0
    best_child: Optional[int] = None
    best_descendant: Optional[int] = None
    
@dataclass 
class ProtoArray:
    """Proto-Array 主结构"""
    nodes: List[ProtoNode] = field(default_factory=list)
    indices: Dict[bytes, int] = field(default_factory=dict)
    finalized_root: bytes = b''
    
    def on_block(self, root: bytes, parent_root: bytes, slot: int):
        """添加新区块"""
        parent_index = self.indices.get(parent_root)
        if parent_index is None and len(self.nodes) > 0:
            raise ValueError("Parent not found")
        
        new_index = len(self.nodes)
        node = ProtoNode(
            root=root,
            parent=parent_index,
            slot=slot,
            best_descendant=new_index  # 叶子节点指向自己
        )
        
        self.nodes.append(node)
        self.indices[root] = new_index
        
        # 向上更新
        if parent_index is not None:
            self._update_ancestor(parent_index)
    
    def apply_score_changes(self, deltas: Dict[bytes, int]):
        """批量应用权重变化"""
        # 收集所有需要更新的节点
        affected_indices = set()
        
        for root, delta in deltas.items():
            index = self.indices.get(root)
            if index is None:
                continue
            
            # 沿路径向上更新权重
            current = index
            while current is not None:
                self.nodes[current].weight += delta
                affected_indices.add(current)
                current = self.nodes[current].parent
        
        # 从叶子向根更新 best_child/best_descendant
        for index in sorted(affected_indices, reverse=True):
            self._update_best_child_and_descendant(index)
    
    def get_head(self) -> bytes:
        """获取链头"""
        if not self.nodes:
            return b''
        
        # 找到 finalized/justified 节点
        start_index = self.indices.get(self.finalized_root, 0)
        
        best_descendant = self.nodes[start_index].best_descendant
        if best_descendant is not None:
            return self.nodes[best_descendant].root
        return self.nodes[start_index].root
    
    def prune(self, new_finalized_root: bytes):
        """剪枝"""
        new_finalized_index = self.indices.get(new_finalized_root)
        if new_finalized_index is None or new_finalized_index == 0:
            return
        
        offset = new_finalized_index
        self.nodes = self.nodes[offset:]
        
        # 更新索引
        for node in self.nodes:
            if node.parent is not None:
                node.parent = node.parent - offset if node.parent >= offset else None
            if node.best_child is not None:
                node.best_child -= offset
            if node.best_descendant is not None:
                node.best_descendant -= offset
        
        self.indices = {node.root: i for i, node in enumerate(self.nodes)}
        self.finalized_root = new_finalized_root
    
    def _update_ancestor(self, index: int):
        """向上更新祖先的 best 字段"""
        current = index
        while current is not None:
            self._update_best_child_and_descendant(current)
            current = self.nodes[current].parent
    
    def _update_best_child_and_descendant(self, index: int):
        """更新节点的 best_child 和 best_descendant"""
        node = self.nodes[index]
        
        # 找所有子节点
        children = [
            i for i, n in enumerate(self.nodes) 
            if n.parent == index
        ]
        
        if not children:
            node.best_child = None
            node.best_descendant = index
            return
        
        # 选权重最大的
        best_child_index = max(
            children,
            key=lambda i: (self.nodes[i].weight, self.nodes[i].root)
        )
        
        node.best_child = best_child_index
        node.best_descendant = self.nodes[best_child_index].best_descendant
```

### 5.2 使用示例

```python
# 创建 Proto-Array
pa = ProtoArray()

# 添加创世块
genesis = b'genesis'
pa.on_block(genesis, b'', slot=0)
pa.finalized_root = genesis

# 构建区块树
#       Genesis
#       /     \
#      A       B
#     / \       \
#    C   D       E

block_a = sha256(b'A').digest()
block_b = sha256(b'B').digest()
block_c = sha256(b'C').digest()
block_d = sha256(b'D').digest()
block_e = sha256(b'E').digest()

pa.on_block(block_a, genesis, slot=1)
pa.on_block(block_b, genesis, slot=1)
pa.on_block(block_c, block_a, slot=2)
pa.on_block(block_d, block_a, slot=2)
pa.on_block(block_e, block_b, slot=2)

# 应用投票权重
pa.apply_score_changes({
    block_c: 100,  # C 获得 100 权重
    block_d: 50,   # D 获得 50 权重
    block_e: 120,  # E 获得 120 权重
})

# 获取链头
head = pa.get_head()
print(f"Head: {head.hex()[:8]}...")  # 应该是 E（权重最大）

# 打印树结构
for i, node in enumerate(pa.nodes):
    print(f"[{i}] weight={node.weight}, best_child={node.best_child}, "
          f"best_descendant={node.best_descendant}")
```

---

## 6. 迁移到比特币最长链

### 6.1 比特币与以太坊的差异

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      比特币 vs 以太坊分叉选择                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   特性              │  以太坊 PoS            │  比特币 PoW              │
│   ─────────────────┼───────────────────────┼─────────────────────────  │
│   选择标准          │  累积验证者权重       │  累积工作量（难度）       │
│   权重来源          │  证明投票             │  区块本身的难度           │
│   更新频率          │  每秒数千次证明       │  约 10 分钟一个区块       │
│   finalized 概念    │  明确的 finalized    │  概率性（6 确认）         │
│   分叉频率          │  较少                 │  偶发                     │
│                                                                         │
│   可复用的思想：                                                        │
│   ✓ 树展平为数组                                                       │
│   ✓ 索引替代哈希查找                                                   │
│   ✓ 预计算最佳后代                                                     │
│   ✓ 剪枝已确认区块                                                     │
│                                                                         │
│   需要调整的部分：                                                      │
│   • 权重计算：从投票累积 → 区块难度累积                                │
│   • 更新时机：从收到证明 → 收到新区块                                  │
│   • 剪枝条件：从 finalized → N 个确认后                                │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 6.2 BitcoinProtoArray 数据结构

```rust
/// 比特币版 Proto-Array 节点
struct BitcoinProtoNode {
    /// 区块哈希
    block_hash: Hash,
    
    /// 父节点索引
    parent: Option<usize>,
    
    /// 区块高度
    height: u64,
    
    /// 该区块的难度（单块）
    difficulty: u256,
    
    /// 从创世块到此区块的累积难度（链工作量）
    chainwork: u256,
    
    /// 最重子节点（累积难度最大）
    best_child: Option<usize>,
    
    /// 该子树中最长链的叶子节点
    best_descendant: Option<usize>,
}

struct BitcoinProtoArray {
    nodes: Vec<BitcoinProtoNode>,
    indices: HashMap<Hash, usize>,
    
    /// 最深的"已确认"区块（6 确认）
    confirmed_depth: u64,
    
    /// 当前最长链的链头
    tip: usize,
}
```

### 6.3 比特币版核心算法

```python
@dataclass
class BitcoinProtoNode:
    """比特币版 Proto-Array 节点"""
    block_hash: bytes
    parent: Optional[int]
    height: int
    difficulty: int          # 单块难度
    chainwork: int           # 累积工作量
    best_child: Optional[int] = None
    best_descendant: Optional[int] = None

class BitcoinProtoArray:
    """比特币最长链选择优化"""
    
    def __init__(self):
        self.nodes: List[BitcoinProtoNode] = []
        self.indices: Dict[bytes, int] = {}
        self.tip: int = 0  # 当前链头索引
        self.confirmed_blocks: int = 6  # 确认数
    
    def on_block(self, block_hash: bytes, parent_hash: bytes, 
                 height: int, difficulty: int):
        """添加新区块"""
        
        parent_index = self.indices.get(parent_hash)
        
        # 计算累积工作量
        if parent_index is not None:
            parent_chainwork = self.nodes[parent_index].chainwork
        else:
            parent_chainwork = 0
        
        chainwork = parent_chainwork + difficulty
        
        # 创建新节点
        new_index = len(self.nodes)
        node = BitcoinProtoNode(
            block_hash=block_hash,
            parent=parent_index,
            height=height,
            difficulty=difficulty,
            chainwork=chainwork,
            best_descendant=new_index  # 叶子节点指向自己
        )
        
        self.nodes.append(node)
        self.indices[block_hash] = new_index
        
        # 向上更新 best_child/best_descendant
        if parent_index is not None:
            self._update_ancestors(parent_index, new_index)
        
        # 检查是否需要切换链头
        self._maybe_update_tip(new_index)
    
    def _update_ancestors(self, parent_index: int, new_child_index: int):
        """向上更新祖先节点"""
        current = parent_index
        child_index = new_child_index
        
        while current is not None:
            node = self.nodes[current]
            child = self.nodes[child_index]
            
            # 检查是否需要更新 best_child
            if node.best_child is None:
                node.best_child = child_index
                node.best_descendant = child.best_descendant
            else:
                current_best = self.nodes[node.best_child]
                # 比较累积工作量
                if child.chainwork > current_best.chainwork or \
                   (child.chainwork == current_best.chainwork and 
                    child.block_hash > current_best.block_hash):
                    node.best_child = child_index
                    node.best_descendant = child.best_descendant
                elif node.best_child == child_index:
                    # best_child 没变，但可能 best_descendant 变了
                    node.best_descendant = child.best_descendant
            
            child_index = current
            current = node.parent
    
    def _maybe_update_tip(self, new_index: int):
        """检查并更新链头"""
        new_node = self.nodes[new_index]
        current_tip = self.nodes[self.tip]
        
        if new_node.chainwork > current_tip.chainwork:
            self.tip = new_index
    
    def get_best_chain_tip(self) -> bytes:
        """获取最长链的链头"""
        # O(1) 操作
        return self.nodes[self.tip].block_hash
    
    def get_chain_at_height(self, height: int) -> Optional[bytes]:
        """获取最长链在指定高度的区块"""
        # 从 tip 向前遍历
        current = self.tip
        while current is not None:
            node = self.nodes[current]
            if node.height == height:
                return node.block_hash
            if node.height < height:
                return None  # 高度超过链长
            current = node.parent
        return None
    
    def prune_confirmed(self):
        """剪枝已确认的区块"""
        if not self.nodes:
            return
        
        tip_height = self.nodes[self.tip].height
        confirm_height = tip_height - self.confirmed_blocks
        
        if confirm_height <= 0:
            return
        
        # 找到确认高度对应的主链区块
        confirmed_index = None
        current = self.tip
        while current is not None:
            if self.nodes[current].height == confirm_height:
                confirmed_index = current
                break
            current = self.nodes[current].parent
        
        if confirmed_index is None or confirmed_index == 0:
            return
        
        # 执行剪枝（与以太坊版类似）
        offset = confirmed_index
        
        # 移除不在主链上的分叉
        main_chain_indices = set()
        current = self.tip
        while current is not None:
            main_chain_indices.add(current)
            current = self.nodes[current].parent
        
        # 只保留主链上 confirmed_index 之后的区块
        new_nodes = []
        old_to_new = {}
        
        for old_idx in sorted(main_chain_indices):
            if old_idx >= confirmed_index:
                new_idx = len(new_nodes)
                old_to_new[old_idx] = new_idx
                new_nodes.append(self.nodes[old_idx])
        
        # 更新索引
        for node in new_nodes:
            if node.parent is not None and node.parent in old_to_new:
                node.parent = old_to_new[node.parent]
            else:
                node.parent = None
            
            if node.best_child is not None and node.best_child in old_to_new:
                node.best_child = old_to_new[node.best_child]
            else:
                node.best_child = None
            
            if node.best_descendant is not None and node.best_descendant in old_to_new:
                node.best_descendant = old_to_new[node.best_descendant]
        
        self.nodes = new_nodes
        self.indices = {node.block_hash: i for i, node in enumerate(self.nodes)}
        self.tip = old_to_new.get(self.tip, len(self.nodes) - 1)
```

### 6.4 处理分叉场景

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      比特币分叉处理示例                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   场景：矿工几乎同时找到两个区块，形成临时分叉                          │
│                                                                         │
│   初始状态：                                                            │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   [Genesis] ← [A] ← [B] ← [C]                                   │  │
│   │                             │                                    │  │
│   │                         chainwork = 300                          │  │
│   │                         tip = C                                  │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   收到分叉区块 D（基于 B）：                                            │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   [Genesis] ← [A] ← [B] ← [C]    chainwork = 300               │  │
│   │                       └── [D]    chainwork = 280               │  │
│   │                                                                  │  │
│   │   tip 仍然 = C（C 的 chainwork 更大）                           │  │
│   │   但 D 被保存，以防后续变长                                      │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   收到 E 和 F（基于 D）：                                               │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   [Genesis] ← [A] ← [B] ← [C]         chainwork = 300          │  │
│   │                       └── [D] ← [E] ← [F]   chainwork = 380    │  │
│   │                                              ▲                  │  │
│   │                                              │                  │  │
│   │   D-E-F 链更长！自动切换 tip = F             │                  │  │
│   │                                              │                  │  │
│   │   Proto-Array 中：                                              │  │
│   │   • D.best_descendant = F                                       │  │
│   │   • B.best_child 从 C 变为 D                                   │  │
│   │   • B.best_descendant = F                                       │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   注意：链切换不需要重新计算所有区块的 chainwork                       │
│   只需要更新 best_child/best_descendant 即可                           │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 6.5 完整比特币示例

```python
# 创建比特币 Proto-Array
btc_pa = BitcoinProtoArray()

# 添加创世块
genesis = bytes.fromhex('000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f')
btc_pa.on_block(genesis, b'', height=0, difficulty=1)

# 构建主链
blocks = [genesis]
for i in range(1, 101):
    block_hash = sha256(f"block_{i}".encode()).digest()
    btc_pa.on_block(block_hash, blocks[-1], height=i, difficulty=1000000)
    blocks.append(block_hash)

print(f"主链高度: {btc_pa.nodes[btc_pa.tip].height}")
print(f"链头: {btc_pa.get_best_chain_tip().hex()[:16]}...")

# 模拟分叉（在高度 95 处分叉）
fork_parent = blocks[95]
for i in range(5):
    fork_hash = sha256(f"fork_{i}".encode()).digest()
    btc_pa.on_block(fork_hash, fork_parent, 
                    height=96+i, 
                    difficulty=1000000)  # 相同难度
    fork_parent = fork_hash

print(f"\n添加分叉后:")
print(f"链头: {btc_pa.get_best_chain_tip().hex()[:16]}...")
print(f"节点总数: {len(btc_pa.nodes)}")

# 分叉链变得更长
for i in range(5, 10):
    fork_hash = sha256(f"fork_{i}".encode()).digest()
    btc_pa.on_block(fork_hash, fork_parent,
                    height=96+i,
                    difficulty=1000000)
    fork_parent = fork_hash

print(f"\n分叉变长后:")
print(f"链头: {btc_pa.get_best_chain_tip().hex()[:16]}...")
print(f"链头高度: {btc_pa.nodes[btc_pa.tip].height}")

# 剪枝
btc_pa.prune_confirmed()
print(f"\n剪枝后节点数: {len(btc_pa.nodes)}")
```

---

## 7. Bitcoin Core 实际实现分析

### 7.1 Bitcoin Core 分叉处理架构

**重要说明**：Bitcoin Core 并未使用 Proto-Array 结构。比特币客户端采用了一套不同但同样高效的分叉管理机制。

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Bitcoin Core 分叉处理架构                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   核心数据结构：                                                        │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   1. CBlockIndex                                                 │  │
│   │      • 每个已知区块的索引信息                                   │  │
│   │      • 包含区块头、累积工作量、父指针等                         │  │
│   │                                                                  │  │
│   │   2. BlockMap (mapBlockIndex)                                    │  │
│   │      • unordered_map<uint256, CBlockIndex*>                     │  │
│   │      • 区块哈希 → 索引的映射                                    │  │
│   │                                                                  │  │
│   │   3. CChain (m_chain / chainActive)                              │  │
│   │      • vector<CBlockIndex*>                                      │  │
│   │      • 当前活跃链，按高度索引                                   │  │
│   │                                                                  │  │
│   │   4. setBlockIndexCandidates                                     │  │
│   │      • set<CBlockIndex*, CBlockIndexWorkComparator>             │  │
│   │      • 候选链头集合，按累积工作量排序                           │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.2 CBlockIndex 结构详解

```cpp
// 简化的 CBlockIndex 结构（来自 Bitcoin Core chain.h）
class CBlockIndex {
public:
    // 区块头哈希
    uint256 phashBlock;
    
    // 父区块指针（链接树结构）
    CBlockIndex* pprev;
    
    // 跳跃指针（用于快速祖先查找）
    CBlockIndex* pskip;
    
    // 区块高度
    int nHeight;
    
    // 区块头信息
    int32_t nVersion;
    uint256 hashMerkleRoot;
    uint32_t nTime;
    uint32_t nBits;     // 难度目标
    uint32_t nNonce;
    
    // 累积工作量（关键字段）
    arith_uint256 nChainWork;
    
    // 验证状态
    uint32_t nStatus;
    
    // 用于快速祖先查找
    CBlockIndex* GetAncestor(int height);
};
```

**关键设计**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      CBlockIndex 指针结构                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   pprev 指针（单向链表）：                                              │
│   ┌───────┐   ┌───────┐   ┌───────┐   ┌───────┐                        │
│   │Block 0│◄──│Block 1│◄──│Block 2│◄──│Block 3│                        │
│   └───────┘   └───────┘   └───────┘   └───────┘                        │
│                                                                         │
│   pskip 跳跃指针（加速祖先查找）：                                      │
│   ┌───────┐   ┌───────┐   ┌───────┐   ┌───────┐                        │
│   │Block 0│◄──────────────│Block 2│   │Block 3│                        │
│   └───────┘   └───────┘   └───────┘   └───┬───┘                        │
│       ▲                                    │                            │
│       └────────────────────────────────────┘                            │
│                    pskip                                                │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.2.1 pskip 跳跃指针详解

pskip 是 Bitcoin Core 中一个精巧的优化，借鉴了 **二进制索引树（Binary Indexed Tree / Fenwick Tree）** 的思想。

**核心公式**：
```
pskip 指向高度 = height - lowbit(height)
其中 lowbit(x) = x & (-x) = x 的二进制最低位 1 代表的值
```

**lowbit 函数解析**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      lowbit(x) = x & (-x)                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   示例计算：                                                            │
│   ┌─────────┬──────────────┬───────────────┬──────────┬─────────────┐  │
│   │ height  │   二进制      │  -height      │ lowbit   │ pskip高度   │  │
│   ├─────────┼──────────────┼───────────────┼──────────┼─────────────┤  │
│   │    1    │   0001       │   1111 (补码) │    1     │   1-1 = 0   │  │
│   │    2    │   0010       │   1110        │    2     │   2-2 = 0   │  │
│   │    3    │   0011       │   1101        │    1     │   3-1 = 2   │  │
│   │    4    │   0100       │   1100        │    4     │   4-4 = 0   │  │
│   │    5    │   0101       │   1011        │    1     │   5-1 = 4   │  │
│   │    6    │   0110       │   1010        │    2     │   6-2 = 4   │  │
│   │    7    │   0111       │   1001        │    1     │   7-1 = 6   │  │
│   │    8    │   1000       │   1000        │    8     │   8-8 = 0   │  │
│   │   12    │   1100       │   0100        │    4     │  12-4 = 8   │  │
│   │   15    │   1111       │   0001        │    1     │  15-1 = 14  │  │
│   └─────────┴──────────────┴───────────────┴──────────┴─────────────┘  │
│                                                                         │
│   规律：                                                                │
│   • 高度为 2^n 的区块，pskip 直接指向创世块                            │
│   • 奇数高度区块，pskip 只跳一步                                       │
│   • 跳跃距离 = lowbit(height)                                          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**pskip 网络结构**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      pskip 跳跃网络（高度 0-15）                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   高度:  0    1    2    3    4    5    6    7    8    ...   15         │
│         [0]  [1]  [2]  [3]  [4]  [5]  [6]  [7]  [8]  ...  [15]         │
│          │    │    │    │    │    │    │    │    │         │           │
│          │    │    │    │    │    │    │    │    │         │           │
│   pskip: │   ─┘   ─┴────┘   ─┴────┴────┴────┘   ─┴─────────┘           │
│          │         │              │              │                      │
│          ◄─────────┘              │              │                      │
│          ◄────────────────────────┘              │                      │
│          ◄───────────────────────────────────────┘                      │
│                                                                         │
│   可视化 pskip 指向：                                                   │
│                                                                         │
│   [15] ─pskip→ [14] ─pskip→ [12] ─pskip→ [8] ─pskip→ [0]              │
│    │            │            │            │                             │
│    └──lowbit=1  └──lowbit=2  └──lowbit=4  └──lowbit=8                  │
│                                                                         │
│   从高度 15 到高度 0：只需 4 跳（而非 15 跳）                          │
│   复杂度：O(log n)                                                      │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**GetAncestor 算法**：

```cpp
// Bitcoin Core 中的实际实现（简化版）
CBlockIndex* CBlockIndex::GetAncestor(int height) {
    if (height > nHeight || height < 0)
        return nullptr;
    
    CBlockIndex* pindex = this;
    int heightWalk = nHeight;
    
    while (heightWalk > height) {
        int heightSkip = GetSkipHeight(heightWalk);
        int heightSkipPrev = GetSkipHeight(heightWalk - 1);
        
        // 如果 pskip 能让我们更接近目标（但不越过）
        if (pindex->pskip != nullptr &&
            (heightSkip == height ||
             (heightSkip > height && !(heightSkipPrev < heightSkip - 2 &&
                                       heightSkipPrev >= height)))) {
            // 使用 pskip 跳跃
            pindex = pindex->pskip;
            heightWalk = heightSkip;
        } else {
            // 使用 pprev 单步后退
            pindex = pindex->pprev;
            heightWalk--;
        }
    }
    return pindex;
}

// 计算跳跃目标高度
inline int GetSkipHeight(int height) {
    if (height < 2)
        return 0;
    // lowbit 操作
    return height - (height & -height);
}
```

**查找示例：从高度 100 找高度 37 的祖先**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      GetAncestor(37) 执行过程                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   起点: height = 100 (二进制: 1100100)                                 │
│   目标: height = 37  (二进制: 0100101)                                 │
│                                                                         │
│   步骤    当前高度    lowbit    跳跃目标    操作                        │
│   ─────────────────────────────────────────────────────────────────     │
│    1       100         4         96       pskip → 96                   │
│    2        96        32         64       pskip → 64                   │
│    3        64        64          0       太远! pprev → 63             │
│    4        63         1         62       pskip → 62                   │
│    5        62         2         60       太远! pprev → 61             │
│    6        61         1         60       太远! pprev → 60             │
│    7        60         4         56       太远! pprev → 59             │
│    ...     (继续混合使用 pskip 和 pprev)                               │
│    n        38         2         36       太近! pprev → 37 ✓           │
│                                                                         │
│   实际跳数: ~14 次（而非 63 次 pprev）                                 │
│   复杂度: O(log(100-37)) = O(log 63) ≈ 6                               │
│                                                                         │
│   注：实际实现会更智能地选择 pskip 或 pprev                            │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.2.2 pskip 插入过程详解

当新区块加入链时，如何计算它的 pskip 应该指向谁？

**插入算法**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      pskip 插入计算                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   新区块高度 = H                                                        │
│   pskip 目标高度 = H - lowbit(H)                                        │
│                                                                         │
│   关键问题：如何快速找到目标高度的区块？                                │
│                                                                         │
│   答案：利用父区块的 pskip 链！                                         │
│                                                                         │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   设新区块高度 = H，父区块 = parent (高度 H-1)                  │  │
│   │   目标高度 = skip_height = H - lowbit(H)                        │  │
│   │                                                                  │  │
│   │   情况 1: skip_height == H - 1                                   │  │
│   │           pskip = parent                                         │  │
│   │                                                                  │  │
│   │   情况 2: skip_height < H - 1                                    │  │
│   │           pskip = parent->GetAncestor(skip_height)               │  │
│   │           (利用父区块的 pskip 链快速找到)                        │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**逐步插入示例：构建高度 0-8 的链**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      逐步插入过程                                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   ═══════════════════════════════════════════════════════════════════  │
│   插入 Block 0 (Genesis)                                                │
│   ═══════════════════════════════════════════════════════════════════  │
│                                                                         │
│   高度 0，无父区块，pskip = NULL                                        │
│                                                                         │
│       [0]                                                               │
│        │                                                                │
│       NULL                                                              │
│                                                                         │
│   ═══════════════════════════════════════════════════════════════════  │
│   插入 Block 1                                                          │
│   ═══════════════════════════════════════════════════════════════════  │
│                                                                         │
│   H = 1, lowbit(1) = 1                                                  │
│   skip_height = 1 - 1 = 0                                               │
│   pskip → Block 0                                                       │
│                                                                         │
│       [0] ◄─── [1]                                                      │
│              pskip                                                      │
│                                                                         │
│   ═══════════════════════════════════════════════════════════════════  │
│   插入 Block 2                                                          │
│   ═══════════════════════════════════════════════════════════════════  │
│                                                                         │
│   H = 2, lowbit(2) = 2 (二进制 10)                                      │
│   skip_height = 2 - 2 = 0                                               │
│   pskip → Block 0                                                       │
│                                                                         │
│       [0] ◄─────────── [2]                                              │
│        ▲               /                                                │
│        └─────── [1] ◄─┘                                                 │
│               pprev                                                     │
│                                                                         │
│   ═══════════════════════════════════════════════════════════════════  │
│   插入 Block 3                                                          │
│   ═══════════════════════════════════════════════════════════════════  │
│                                                                         │
│   H = 3, lowbit(3) = 1 (二进制 11)                                      │
│   skip_height = 3 - 1 = 2                                               │
│   pskip → Block 2                                                       │
│                                                                         │
│       [0] ◄───── [2] ◄───── [3]                                         │
│        ▲          ▲        /                                            │
│        │          └───────┘ pskip                                       │
│        └─ [1] ◄──┘                                                      │
│                                                                         │
│   ═══════════════════════════════════════════════════════════════════  │
│   插入 Block 4                                                          │
│   ═══════════════════════════════════════════════════════════════════  │
│                                                                         │
│   H = 4, lowbit(4) = 4 (二进制 100)                                     │
│   skip_height = 4 - 4 = 0                                               │
│   pskip → Block 0 (跨越整个链！)                                        │
│                                                                         │
│       [0] ◄─────────────────────────────── [4]                          │
│        ▲                                  /                             │
│        │    [2] ◄─────── [3] ◄──────────┘ pprev                        │
│        │     ▲                                                          │
│        └─ [1]                                                           │
│                                                                         │
│   ═══════════════════════════════════════════════════════════════════  │
│   插入 Block 5                                                          │
│   ═══════════════════════════════════════════════════════════════════  │
│                                                                         │
│   H = 5, lowbit(5) = 1 (二进制 101)                                     │
│   skip_height = 5 - 1 = 4                                               │
│   pskip → Block 4                                                       │
│                                                                         │
│              [4] ◄───── [5]                                             │
│               ▲        /                                                │
│               └───────┘ pskip                                           │
│                                                                         │
│   ═══════════════════════════════════════════════════════════════════  │
│   插入 Block 6                                                          │
│   ═══════════════════════════════════════════════════════════════════  │
│                                                                         │
│   H = 6, lowbit(6) = 2 (二进制 110)                                     │
│   skip_height = 6 - 2 = 4                                               │
│   pskip → Block 4                                                       │
│                                                                         │
│              [4] ◄─────────── [6]                                       │
│               ▲              /                                          │
│               └─ [5] ◄──────┘ pprev                                     │
│                                                                         │
│   ═══════════════════════════════════════════════════════════════════  │
│   插入 Block 7                                                          │
│   ═══════════════════════════════════════════════════════════════════  │
│                                                                         │
│   H = 7, lowbit(7) = 1 (二进制 111)                                     │
│   skip_height = 7 - 1 = 6                                               │
│   pskip → Block 6                                                       │
│                                                                         │
│                     [6] ◄───── [7]                                      │
│                      ▲        /                                         │
│                      └───────┘ pskip                                    │
│                                                                         │
│   ═══════════════════════════════════════════════════════════════════  │
│   插入 Block 8                                                          │
│   ═══════════════════════════════════════════════════════════════════  │
│                                                                         │
│   H = 8, lowbit(8) = 8 (二进制 1000)                                    │
│   skip_height = 8 - 8 = 0                                               │
│   pskip → Block 0 (直接跳到创世块！)                                    │
│                                                                         │
│       [0] ◄───────────────────────────────────────── [8]                │
│        ▲                                            /                   │
│        │    [4] ◄─────────── [6] ◄─ [7] ◄──────────┘ pprev             │
│        │     ▲                                                          │
│        │     │                                                          │
│        │    [2] ◄─ [3]                                                  │
│        │     ▲                                                          │
│        └─ [1]                                                           │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**完整的 pskip 网络图（高度 0-15）**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      完整 pskip 网络（高度 0-15）                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   pprev 链（常规父子关系）：                                            │
│   [0]←[1]←[2]←[3]←[4]←[5]←[6]←[7]←[8]←[9]←[10]←[11]←[12]←[13]←[14]←[15]│
│                                                                         │
│   pskip 跳跃指针：                                                      │
│                                                                         │
│   高度  二进制   lowbit  skip到                                         │
│   ────────────────────────────                                          │
│    0    0000      -      NULL                                           │
│    1    0001      1      → 0    ─────────────────────────────────────┐  │
│    2    0010      2      → 0    ─────────────────────────────────────┤  │
│    3    0011      1      → 2    ──────────────────────────────┐      │  │
│    4    0100      4      → 0    ─────────────────────────────────────┤  │
│    5    0101      1      → 4    ───────────────────────┐      │      │  │
│    6    0110      2      → 4    ───────────────────────┤      │      │  │
│    7    0111      1      → 6    ────────────────┐      │      │      │  │
│    8    1000      8      → 0    ─────────────────────────────────────┤  │
│    9    1001      1      → 8    ────────┐      │      │      │      │  │
│   10    1010      2      → 8    ────────┤      │      │      │      │  │
│   11    1011      1      → 10   ──┐     │      │      │      │      │  │
│   12    1100      4      → 8    ────────┤      │      │      │      │  │
│   13    1101      1      → 12   ──┐     │      │      │      │      │  │
│   14    1110      2      → 12   ──┤     │      │      │      │      │  │
│   15    1111      1      → 14   ┐ │     │      │      │      │      │  │
│                                 │ │     │      │      │      │      │  │
│                                 │ │     │      │      │      │      │  │
│                                 ▼ ▼     ▼      ▼      ▼      ▼      ▼  │
│                                                                         │
│   可视化跳跃：                                                          │
│                                                                         │
│   层级 1（跳 1 步）:  1→0, 3→2, 5→4, 7→6, 9→8, 11→10, 13→12, 15→14    │
│   层级 2（跳 2 步）:  2→0, 6→4, 10→8, 14→12                            │
│   层级 4（跳 4 步）:  4→0, 12→8                                        │
│   层级 8（跳 8 步）:  8→0                                              │
│                                                                         │
│   形成类似二叉树的结构，任意节点到 0 最多 log₂(n) 跳                   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.2.3 pskip 查找过程详解

**查找算法核心思想**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      GetAncestor 查找策略                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   目标：从高度 H 找到高度 T 的祖先（T < H）                             │
│                                                                         │
│   策略：每一步选择"不越过目标的最大跳跃"                               │
│                                                                         │
│   while (当前高度 > 目标高度):                                          │
│       skip_target = 当前高度 - lowbit(当前高度)                         │
│                                                                         │
│       if skip_target >= 目标高度:                                       │
│           使用 pskip（大跳）                                            │
│       else:                                                             │
│           使用 pprev（小步）                                            │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**查找示例 1：从高度 15 找高度 0**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      GetAncestor(15 → 0)                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   起点: [15]    目标: [0]                                               │
│                                                                         │
│   步骤 1: 高度=15, lowbit(15)=1, skip到14                               │
│           14 >= 0? ✓ 使用 pskip                                         │
│                                                                         │
│   [0]                            [14] ◄── [15]                          │
│    ▲                              │      pskip                          │
│    │                              │                                     │
│    └──────────────────────────────┘ (待跳)                              │
│                                                                         │
│   步骤 2: 高度=14, lowbit(14)=2, skip到12                               │
│           12 >= 0? ✓ 使用 pskip                                         │
│                                                                         │
│   [0]                    [12] ◄── [14]                                  │
│    ▲                      │      pskip                                  │
│    │                      │                                             │
│    └──────────────────────┘ (待跳)                                      │
│                                                                         │
│   步骤 3: 高度=12, lowbit(12)=4, skip到8                                │
│           8 >= 0? ✓ 使用 pskip                                          │
│                                                                         │
│   [0]            [8] ◄── [12]                                           │
│    ▲              │     pskip                                           │
│    │              │                                                     │
│    └──────────────┘ (待跳)                                              │
│                                                                         │
│   步骤 4: 高度=8, lowbit(8)=8, skip到0                                  │
│           0 >= 0? ✓ 使用 pskip                                          │
│                                                                         │
│   [0] ◄───────── [8]                                                    │
│        pskip                                                            │
│                                                                         │
│   完成！只用了 4 跳（而非 15 次 pprev）                                 │
│                                                                         │
│   路径: 15 →(pskip)→ 14 →(pskip)→ 12 →(pskip)→ 8 →(pskip)→ 0          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**查找示例 2：从高度 15 找高度 5**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      GetAncestor(15 → 5)                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   起点: [15]    目标: [5]                                               │
│                                                                         │
│   步骤 1: 高度=15, lowbit=1, skip到14                                   │
│           14 >= 5? ✓ pskip → [14]                                       │
│                                                                         │
│   步骤 2: 高度=14, lowbit=2, skip到12                                   │
│           12 >= 5? ✓ pskip → [12]                                       │
│                                                                         │
│   步骤 3: 高度=12, lowbit=4, skip到8                                    │
│           8 >= 5? ✓ pskip → [8]                                         │
│                                                                         │
│   步骤 4: 高度=8, lowbit=8, skip到0                                     │
│           0 >= 5? ✗ 会越过目标！使用 pprev → [7]                        │
│                                                                         │
│   步骤 5: 高度=7, lowbit=1, skip到6                                     │
│           6 >= 5? ✓ pskip → [6]                                         │
│                                                                         │
│   步骤 6: 高度=6, lowbit=2, skip到4                                     │
│           4 >= 5? ✗ 会越过目标！使用 pprev → [5]                        │
│                                                                         │
│   完成！高度=5，找到目标                                                │
│                                                                         │
│   路径: 15 → 14 → 12 → 8 → 7 → 6 → 5                                   │
│              pskip pskip pskip pprev pskip pprev                        │
│                                                                         │
│   共 6 步（而非 10 次 pprev）                                           │
│                                                                         │
│   可视化：                                                              │
│                                                                         │
│   [5] ◄─ [6] ◄─ [7] ◄─ [8] ◄─ [12] ◄─ [14] ◄─ [15]                    │
│       pprev pskip pprev pskip  pskip  pskip                             │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**查找示例 3：从高度 100 找高度 37（大跨度）**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      GetAncestor(100 → 37)                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   100 = 1100100 (二进制)                                                │
│   37  = 0100101 (二进制)                                                │
│                                                                         │
│   步骤    当前   二进制      lowbit  skip到  >=37?  操作       跳到     │
│   ─────────────────────────────────────────────────────────────────     │
│    1      100    1100100       4      96      ✓    pskip       96      │
│    2       96    1100000      32      64      ✓    pskip       64      │
│    3       64    1000000      64       0      ✗    pprev       63      │
│    4       63    0111111       1      62      ✓    pskip       62      │
│    5       62    0111110       2      60      ✓    pskip       60      │
│    6       60    0111100       4      56      ✓    pskip       56      │
│    7       56    0111000       8      48      ✓    pskip       48      │
│    8       48    0110000      16      32      ✗    pprev       47      │
│    9       47    0101111       1      46      ✓    pskip       46      │
│   10       46    0101110       2      44      ✓    pskip       44      │
│   11       44    0101100       4      40      ✓    pskip       40      │
│   12       40    0101000       8      32      ✗    pprev       39      │
│   13       39    0100111       1      38      ✓    pskip       38      │
│   14       38    0100110       2      36      ✗    pprev       37 ✓   │
│                                                                         │
│   共 14 步（而非 63 次 pprev）                                          │
│   理论最优: O(log₂(100-37)) = O(log₂63) ≈ 6                            │
│   实际略多因为需要混合 pskip 和 pprev                                   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.2.4 插入时如何找到 pskip 目标？

**关键优化**：插入时利用父区块的 pskip 链

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      插入时设置 pskip                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   新区块高度 H，需要找高度 (H - lowbit(H)) 的区块设为 pskip             │
│                                                                         │
│   方法：调用 parent->GetAncestor(H - lowbit(H))                         │
│                                                                         │
│   示例：插入高度 100 的区块                                             │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   H = 100, lowbit(100) = 4                                       │  │
│   │   skip_height = 100 - 4 = 96                                     │  │
│   │                                                                  │  │
│   │   parent = Block 99                                              │  │
│   │   Block 100.pskip = Block 99->GetAncestor(96)                   │  │
│   │                                                                  │  │
│   │   GetAncestor(96) 只需要几步（O(log n)）                        │  │
│   │   99 → 98 → 96  (3步)                                           │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   总插入开销: O(log n)                                                  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

```cpp
// Bitcoin Core 实际的 pskip 设置代码
void CBlockIndex::BuildSkip() {
    if (pprev) {
        // 利用父区块快速找到目标祖先
        pskip = pprev->GetAncestor(GetSkipHeight(nHeight));
    }
}

// GetSkipHeight 计算跳跃目标
static inline int GetSkipHeight(int height) {
    if (height < 2)
        return 0;
    // height - lowbit(height)
    return height - (height & -height);
}
```

**Python 模拟实现**：

```python
def lowbit(x):
    """计算 x 的最低位 1 代表的值"""
    return x & (-x)

def get_skip_height(height):
    """计算 pskip 指向的高度"""
    if height < 2:
        return 0
    return height - lowbit(height)

class BlockIndex:
    def __init__(self, height, pprev=None):
        self.height = height
        self.pprev = pprev
        # pskip 指向 height - lowbit(height) 处的祖先
        self.pskip = None
        
    def setup_skip(self, chain):
        """设置 pskip 指针"""
        skip_height = get_skip_height(self.height)
        if skip_height >= 0 and skip_height < len(chain):
            self.pskip = chain[skip_height]
    
    def get_ancestor(self, target_height):
        """O(log n) 查找指定高度的祖先"""
        if target_height > self.height or target_height < 0:
            return None
        
        current = self
        while current.height > target_height:
            skip_height = get_skip_height(current.height)
            
            # 判断是否应该使用 pskip
            if current.pskip and skip_height >= target_height:
                current = current.pskip
            else:
                current = current.pprev
        
        return current

# 构建链并测试
def build_chain(length):
    chain = [BlockIndex(0)]  # Genesis
    for h in range(1, length):
        block = BlockIndex(h, chain[h-1])
        chain.append(block)
        block.setup_skip(chain)
    return chain

# 测试
chain = build_chain(1000)

# 从高度 999 查找高度 123 的祖先
ancestor = chain[999].get_ancestor(123)
print(f"Found ancestor at height: {ancestor.height}")  # 输出: 123

# 统计跳跃次数
def get_ancestor_with_count(block, target_height):
    """带跳跃计数的版本"""
    jumps = 0
    current = block
    while current.height > target_height:
        skip_height = get_skip_height(current.height)
        if current.pskip and skip_height >= target_height:
            current = current.pskip
        else:
            current = current.pprev
        jumps += 1
    return current, jumps

ancestor, jumps = get_ancestor_with_count(chain[999], 123)
print(f"Jumps needed: {jumps}")  # 约 18 次（vs 876 次纯 pprev）
```

**为什么这个设计有效？**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      pskip 设计原理                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   二进制视角：                                                          │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   任何正整数都可以表示为 2^a + 2^b + 2^c + ... 的形式           │  │
│   │   例如: 100 = 64 + 32 + 4 = 2^6 + 2^5 + 2^2                     │  │
│   │                                                                  │  │
│   │   pskip 每次消除最低位的 1                                       │  │
│   │   100 (1100100) → 96 (1100000) → 64 (1000000) → 0               │  │
│   │                                                                  │  │
│   │   最多需要 log₂(n) 次 pskip 就能到达任意祖先                    │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.2.5 背景知识：Binary Indexed Tree（树状数组）

pskip 的设计灵感来源于 **Binary Indexed Tree（BIT）**，也称为 **Fenwick Tree**。这是一种经典的数据结构，由 Peter Fenwick 于 1994 年发明。

**BIT 的核心问题**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      前缀和问题                                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   给定数组 A[1..n]，需要支持两种操作：                                  │
│                                                                         │
│   1. 单点更新: A[i] += delta                                            │
│   2. 前缀查询: sum(A[1..k]) = A[1] + A[2] + ... + A[k]                 │
│                                                                         │
│   朴素方法：                                                            │
│   ┌───────────────────────────────────────────────────────────────┐    │
│   │  更新: O(1)                                                    │    │
│   │  查询: O(n) —— 需要遍历求和                                    │    │
│   └───────────────────────────────────────────────────────────────┘    │
│                                                                         │
│   前缀和数组：                                                          │
│   ┌───────────────────────────────────────────────────────────────┐    │
│   │  更新: O(n) —— 需要更新所有后续前缀和                          │    │
│   │  查询: O(1)                                                    │    │
│   └───────────────────────────────────────────────────────────────┘    │
│                                                                         │
│   BIT（树状数组）：                                                     │
│   ┌───────────────────────────────────────────────────────────────┐    │
│   │  更新: O(log n)                                                │    │
│   │  查询: O(log n)                                                │    │
│   │  两种操作都高效！                                              │    │
│   └───────────────────────────────────────────────────────────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**BIT 的核心思想：lowbit 分解**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      lowbit 函数                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   lowbit(x) = x & (-x) = x 的二进制最低位 1 所代表的值                 │
│                                                                         │
│   原理（以 x = 12 为例）：                                              │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   x  = 12 = 0000 1100 (二进制)                                  │  │
│   │   -x = -12 的补码表示                                            │  │
│   │                                                                  │  │
│   │   补码计算：取反 + 1                                             │  │
│   │   ~x     = 1111 0011                                            │  │
│   │   ~x + 1 = 1111 0100 = -12 的补码                               │  │
│   │                                                                  │  │
│   │   x & (-x) = 0000 1100                                          │  │
│   │            & 1111 0100                                          │  │
│   │            ──────────                                           │  │
│   │            = 0000 0100 = 4                                      │  │
│   │                                                                  │  │
│   │   结果：lowbit(12) = 4 = 2^2（最低位 1 的位置是第 2 位）        │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   更多示例：                                                            │
│   ┌─────────┬──────────────┬──────────────┬──────────┐                │
│   │    x    │   二进制      │  lowbit(x)   │  含义    │                │
│   ├─────────┼──────────────┼──────────────┼──────────┤                │
│   │    1    │   0001       │      1       │  2^0     │                │
│   │    2    │   0010       │      2       │  2^1     │                │
│   │    3    │   0011       │      1       │  2^0     │                │
│   │    4    │   0100       │      4       │  2^2     │                │
│   │    5    │   0101       │      1       │  2^0     │                │
│   │    6    │   0110       │      2       │  2^1     │                │
│   │    7    │   0111       │      1       │  2^0     │                │
│   │    8    │   1000       │      8       │  2^3     │                │
│   │   12    │   1100       │      4       │  2^2     │                │
│   └─────────┴──────────────┴──────────────┴──────────┘                │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**BIT 的结构**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      BIT 数组结构（n=16）                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   原数组 A:  [1] [2] [3] [4] [5] [6] [7] [8] [9] [10][11][12][13][14]...│
│                                                                         │
│   BIT[i] 存储的范围：从 (i - lowbit(i) + 1) 到 i 的和                  │
│                                                                         │
│   索引 i   lowbit(i)   BIT[i] 覆盖范围                                 │
│   ─────────────────────────────────────────                             │
│     1         1        A[1]           (1个元素)                        │
│     2         2        A[1..2]        (2个元素)                        │
│     3         1        A[3]           (1个元素)                        │
│     4         4        A[1..4]        (4个元素)                        │
│     5         1        A[5]           (1个元素)                        │
│     6         2        A[5..6]        (2个元素)                        │
│     7         1        A[7]           (1个元素)                        │
│     8         8        A[1..8]        (8个元素)                        │
│     9         1        A[9]           (1个元素)                        │
│    10         2        A[9..10]       (2个元素)                        │
│    11         1        A[11]          (1个元素)                        │
│    12         4        A[9..12]       (4个元素)                        │
│                                                                         │
│   可视化：                                                              │
│                                                                         │
│   层级视图：                                                            │
│                      ┌───────────────────────────────┐                 │
│   BIT[8]:            │         A[1..8]               │                 │
│                      └───────────────────────────────┘                 │
│                      ┌───────────────┐                                 │
│   BIT[4]:            │    A[1..4]    │                                 │
│                      └───────────────┘                                 │
│                      ┌───────┐       ┌───────┐                         │
│   BIT[2], BIT[6]:    │A[1..2]│       │A[5..6]│                         │
│                      └───────┘       └───────┘                         │
│                      ┌───┐┌───┐     ┌───┐┌───┐                         │
│   BIT[1,3,5,7]:      │A1 ││A3 │     │A5 ││A7 │                         │
│                      └───┘└───┘     └───┘└───┘                         │
│                                                                         │
│   索引:               1  2  3  4     5  6  7  8                        │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**BIT 的查询操作**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      BIT 前缀和查询                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   计算 sum(A[1..k])：                                                   │
│                                                                         │
│   def query(k):                                                         │
│       result = 0                                                        │
│       while k > 0:                                                      │
│           result += BIT[k]                                              │
│           k -= lowbit(k)    # 关键：向左跳                             │
│       return result                                                     │
│                                                                         │
│   示例：query(11) = sum(A[1..11])                                      │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   k = 11 (1011)                                                  │  │
│   │       result += BIT[11]     # A[11]                             │  │
│   │       k = 11 - lowbit(11) = 11 - 1 = 10                         │  │
│   │                                                                  │  │
│   │   k = 10 (1010)                                                  │  │
│   │       result += BIT[10]     # A[9..10]                          │  │
│   │       k = 10 - lowbit(10) = 10 - 2 = 8                          │  │
│   │                                                                  │  │
│   │   k = 8 (1000)                                                   │  │
│   │       result += BIT[8]      # A[1..8]                           │  │
│   │       k = 8 - lowbit(8) = 8 - 8 = 0                             │  │
│   │                                                                  │  │
│   │   k = 0, 结束                                                    │  │
│   │                                                                  │  │
│   │   结果: BIT[11] + BIT[10] + BIT[8]                              │  │
│   │       = A[11] + A[9..10] + A[1..8]                              │  │
│   │       = A[1..11] ✓                                               │  │
│   │                                                                  │  │
│   │   只需 3 步！（而非 11 次累加）                                  │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   复杂度：O(log n) —— 最多 log₂(n) 个 1 需要消除                       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**BIT 的更新操作**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      BIT 单点更新                                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   更新 A[i] += delta：                                                  │
│                                                                         │
│   def update(i, delta):                                                 │
│       while i <= n:                                                     │
│           BIT[i] += delta                                               │
│           i += lowbit(i)    # 关键：向右跳                             │
│       return                                                            │
│                                                                         │
│   示例：update(3, +5)                                                   │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   i = 3 (0011)                                                   │  │
│   │       BIT[3] += 5       # 包含 A[3]                             │  │
│   │       i = 3 + lowbit(3) = 3 + 1 = 4                             │  │
│   │                                                                  │  │
│   │   i = 4 (0100)                                                   │  │
│   │       BIT[4] += 5       # 包含 A[1..4]，含 A[3]                 │  │
│   │       i = 4 + lowbit(4) = 4 + 4 = 8                             │  │
│   │                                                                  │  │
│   │   i = 8 (1000)                                                   │  │
│   │       BIT[8] += 5       # 包含 A[1..8]，含 A[3]                 │  │
│   │       i = 8 + lowbit(8) = 8 + 8 = 16                            │  │
│   │                                                                  │  │
│   │   i = 16 (假设 n=16)                                             │  │
│   │       BIT[16] += 5                                              │  │
│   │       i = 32 > n, 结束                                          │  │
│   │                                                                  │  │
│   │   更新了: BIT[3], BIT[4], BIT[8], BIT[16]                       │  │
│   │   这些都是包含 A[3] 的节点                                      │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   复杂度：O(log n)                                                      │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**BIT 完整实现**：

```python
class BinaryIndexedTree:
    """树状数组（Binary Indexed Tree / Fenwick Tree）"""
    
    def __init__(self, n):
        self.n = n
        self.tree = [0] * (n + 1)  # 1-indexed
    
    @staticmethod
    def lowbit(x):
        """返回 x 的最低位 1 代表的值"""
        return x & (-x)
    
    def update(self, i, delta):
        """单点更新：A[i] += delta"""
        while i <= self.n:
            self.tree[i] += delta
            i += self.lowbit(i)  # 向右（向上）跳
    
    def query(self, k):
        """前缀查询：sum(A[1..k])"""
        result = 0
        while k > 0:
            result += self.tree[k]
            k -= self.lowbit(k)  # 向左（向下）跳
        return result
    
    def range_query(self, left, right):
        """区间查询：sum(A[left..right])"""
        return self.query(right) - self.query(left - 1)

# 使用示例
bit = BinaryIndexedTree(16)

# 初始化数组 [1, 2, 3, 4, 5, 6, 7, 8, ...]
for i in range(1, 17):
    bit.update(i, i)

print(f"sum(1..8) = {bit.query(8)}")    # 1+2+...+8 = 36
print(f"sum(1..11) = {bit.query(11)}")  # 1+2+...+11 = 66
print(f"sum(5..10) = {bit.range_query(5, 10)}")  # 5+6+...+10 = 45
```

**BIT 与 pskip 的对比**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      BIT 与 pskip 的联系                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   核心相似点：都使用 lowbit 实现 O(log n) 跳跃                          │
│                                                                         │
│   ┌─────────────────────────┬─────────────────────────────────────────┐│
│   │      BIT 查询           │           pskip 查询                    ││
│   ├─────────────────────────┼─────────────────────────────────────────┤│
│   │ k -= lowbit(k)          │  height -= lowbit(height)              ││
│   │ 累加 BIT[k]             │  跟随 pskip 指针                       ││
│   │ 直到 k = 0              │  直到 height = target                  ││
│   └─────────────────────────┴─────────────────────────────────────────┘│
│                                                                         │
│   ┌─────────────────────────┬─────────────────────────────────────────┐│
│   │      BIT 更新           │           pskip 插入                    ││
│   ├─────────────────────────┼─────────────────────────────────────────┤│
│   │ i += lowbit(i)          │  找 parent.GetAncestor(h-lowbit(h))    ││
│   │ 更新所有覆盖 i 的节点    │  设置一个 pskip 指针                   ││
│   │ 向右（向上）传播         │  O(log n) 找目标                       ││
│   └─────────────────────────┴─────────────────────────────────────────┘│
│                                                                         │
│   关键洞察：                                                            │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   BIT 利用 lowbit 将数组"分层"：                                │  │
│   │   • 奇数位置（lowbit=1）：覆盖 1 个元素                         │  │
│   │   • 偶数位置（lowbit=2）：覆盖 2 个元素                         │  │
│   │   • 4 的倍数（lowbit=4）：覆盖 4 个元素                         │  │
│   │   • 2^k 的倍数：覆盖 2^k 个元素                                 │  │
│   │                                                                  │  │
│   │   pskip 利用相同的分层思想：                                     │  │
│   │   • 奇数高度：跳 1 步                                           │  │
│   │   • 偶数高度：跳 2 步                                           │  │
│   │   • 4 的倍数高度：跳 4 步                                       │  │
│   │   • 2^k 的倍数高度：跳 2^k 步（直达远祖先）                     │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   这种分层确保：任意查询/祖先查找最多 log₂(n) 步                       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**BIT 的数学原理：递归视角**

用递归公式来理解 BIT 是最简洁的方式。

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      核心递归公式                                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   定义：sum(i) = A[1] + A[2] + ... + A[i]                              │
│                                                                         │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   递归公式：                                                     │  │
│   │                                                                  │  │
│   │       sum(0) = 0                          （基础情况）           │  │
│   │                                                                  │  │
│   │       sum(i) = sum(i - lowbit(i)) + BIT[i]  （递归情况）         │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   这就是全部！整个 BIT 查询算法就是这个递归公式的直接实现。            │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**为什么递归公式成立？**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      递归公式的正确性                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   BIT[i] 的定义：                                                       │
│       BIT[i] = A[i - lowbit(i) + 1] + A[i - lowbit(i) + 2] + ... + A[i]│
│              = sum(i) - sum(i - lowbit(i))                             │
│                                                                         │
│   移项得到：                                                            │
│       sum(i) = sum(i - lowbit(i)) + BIT[i]   ✓                         │
│                                                                         │
│   ───────────────────────────────────────────────────────────────────  │
│                                                                         │
│   图示：                                                                │
│                                                                         │
│   sum(i) = [─────────────────────────────────────]                     │
│             1                                   i                       │
│                                                                         │
│          = [─────────────────────] + [────────────]                    │
│             1          i-lowbit(i)   i-lowbit(i)+1   i                 │
│             └── sum(i-lowbit(i)) ─┘  └── BIT[i] ────┘                  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**递归展开示例：sum(11)**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      sum(11) 的递归展开                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   sum(11) = sum(11 - lowbit(11)) + BIT[11]                             │
│           = sum(11 - 1) + BIT[11]                                       │
│           = sum(10) + BIT[11]                                           │
│                                                                         │
│           = sum(10 - lowbit(10)) + BIT[10] + BIT[11]                   │
│           = sum(10 - 2) + BIT[10] + BIT[11]                            │
│           = sum(8) + BIT[10] + BIT[11]                                 │
│                                                                         │
│           = sum(8 - lowbit(8)) + BIT[8] + BIT[10] + BIT[11]            │
│           = sum(8 - 8) + BIT[8] + BIT[10] + BIT[11]                    │
│           = sum(0) + BIT[8] + BIT[10] + BIT[11]                        │
│                                                                         │
│           = 0 + BIT[8] + BIT[10] + BIT[11]                             │
│                                                                         │
│           = BIT[8] + BIT[10] + BIT[11]                                 │
│                                                                         │
│   ───────────────────────────────────────────────────────────────────  │
│                                                                         │
│   每一步递归：                                                          │
│   ┌────────────┬─────────────┬─────────────────────────────────────┐   │
│   │  当前 i    │  i-lowbit(i)│  BIT[i] 覆盖                        │   │
│   ├────────────┼─────────────┼─────────────────────────────────────┤   │
│   │    11      │     10      │  A[11]                              │   │
│   │    10      │      8      │  A[9] + A[10]                       │   │
│   │     8      │      0      │  A[1] + A[2] + ... + A[8]          │   │
│   │     0      │    (终止)    │  —                                 │   │
│   └────────────┴─────────────┴─────────────────────────────────────┘   │
│                                                                         │
│   合并：A[1..8] + A[9..10] + A[11] = A[1..11] ✓                        │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**递归树可视化**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      递归调用树                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   sum(11) = sum(10) + BIT[11]                                          │
│               │                                                         │
│               ▼                                                         │
│           sum(10) = sum(8) + BIT[10]                                   │
│                       │                                                 │
│                       ▼                                                 │
│                   sum(8) = sum(0) + BIT[8]                             │
│                              │                                          │
│                              ▼                                          │
│                          sum(0) = 0  ← 基础情况                        │
│                                                                         │
│   回溯求值：                                                            │
│   sum(0)  = 0                                                          │
│   sum(8)  = 0 + BIT[8]  = BIT[8]                                       │
│   sum(10) = BIT[8] + BIT[10]                                           │
│   sum(11) = BIT[8] + BIT[10] + BIT[11]                                 │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**sum(i) 与 sum(i-1) 的关系**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      sum(i) 与 sum(i-1) 的递归关系                      │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   显然：sum(i) = sum(i-1) + A[i]                                       │
│                                                                         │
│   但这不是 BIT 使用的递归！                                             │
│                                                                         │
│   BIT 的递归：sum(i) = sum(i - lowbit(i)) + BIT[i]                     │
│                                                                         │
│   ───────────────────────────────────────────────────────────────────  │
│                                                                         │
│   对比两种递归：                                                        │
│                                                                         │
│   方法 1：sum(i) = sum(i-1) + A[i]                                     │
│           • 每次只减 1                                                  │
│           • 需要 i 步到达 sum(0)                                       │
│           • 复杂度 O(i)                                                │
│                                                                         │
│   方法 2：sum(i) = sum(i - lowbit(i)) + BIT[i]                         │
│           • 每次减 lowbit(i)（至少减 1，可能减很多）                   │
│           • 最多 log₂(i) 步到达 sum(0)                                 │
│           • 复杂度 O(log i)                                            │
│                                                                         │
│   ───────────────────────────────────────────────────────────────────  │
│                                                                         │
│   关键洞察：                                                            │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   BIT 预先计算并存储了 BIT[i] = sum(i) - sum(i - lowbit(i))     │  │
│   │                                                                  │  │
│   │   这让我们可以"跳过" lowbit(i) 个元素                           │  │
│   │   而不是一个一个累加                                            │  │
│   │                                                                  │  │
│   │   代价：更新时需要维护 BIT 数组                                  │  │
│   │   收益：查询从 O(n) 降到 O(log n)                                │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**为什么 lowbit 跳跃有效？**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      lowbit 跳跃的数学本质                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   核心观察：i - lowbit(i) 消除 i 的最低位 1                            │
│                                                                         │
│   i = 11 = 1011                                                        │
│   i - lowbit(i) = 11 - 1 = 10 = 1010  （消除了最低位的 1）             │
│                                                                         │
│   i = 10 = 1010                                                        │
│   i - lowbit(i) = 10 - 2 = 8 = 1000   （消除了最低位的 1）             │
│                                                                         │
│   i = 8 = 1000                                                         │
│   i - lowbit(i) = 8 - 8 = 0 = 0000    （消除了最后一个 1）             │
│                                                                         │
│   ───────────────────────────────────────────────────────────────────  │
│                                                                         │
│   因为 i 的二进制最多有 log₂(i) 个 1                                   │
│   所以最多 log₂(i) 次操作就能到达 0                                    │
│                                                                         │
│   示例：                                                                │
│   ┌─────────────┬─────────────┬───────────────────────────────────┐    │
│   │     i       │   二进制     │  到 0 的步数                       │    │
│   ├─────────────┼─────────────┼───────────────────────────────────┤    │
│   │     8       │   1000      │    1（只有 1 个 1）                │    │
│   │    11       │   1011      │    3（有 3 个 1）                  │    │
│   │    15       │   1111      │    4（有 4 个 1）                  │    │
│   │   255       │  11111111   │    8（有 8 个 1）                  │    │
│   │   256       │ 100000000   │    1（只有 1 个 1）                │    │
│   └─────────────┴─────────────┴───────────────────────────────────┘    │
│                                                                         │
│   步数 = popcount(i) = i 的二进制中 1 的个数 ≤ log₂(i)                │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**递归公式的代码实现**

```python
def query_recursive(BIT, i):
    """递归版本的 BIT 查询"""
    if i == 0:
        return 0
    return query_recursive(BIT, i - lowbit(i)) + BIT[i]

def query_iterative(BIT, i):
    """迭代版本（尾递归优化）"""
    result = 0
    while i > 0:
        result += BIT[i]
        i -= lowbit(i)
    return result

# 两个版本完全等价！
# 迭代版本就是递归版本的尾递归展开
```

**总结：BIT 的递归本质**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      一句话总结                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   BIT 的本质是一个递归定义：                                           │
│                                                                         │
│       sum(i) = sum(i - lowbit(i)) + BIT[i]                             │
│                                                                         │
│   其中 BIT[i] 预存储了 A[i-lowbit(i)+1..i] 的和                        │
│                                                                         │
│   这个递归每次"消灭" i 的最低位 1                                      │
│   所以最多递归 O(log i) 次                                             │
│                                                                         │
│   pskip 同理：                                                          │
│       GetAncestor(height) = GetAncestor(height - lowbit(height)) 的祖先│
│       然后沿 pskip 跳一步                                              │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**总结：BIT/pskip 的设计精髓**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      设计精髓                                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   1. 利用二进制表示的稀疏性                                             │
│      • n 的二进制最多有 log₂(n) 个 1                                   │
│      • 每次操作消除或添加一个 1                                        │
│      • 因此最多 O(log n) 步                                            │
│                                                                         │
│   2. 空间开销极小                                                       │
│      • BIT：额外 O(n) 空间（与原数组同阶）                             │
│      • pskip：每个节点仅多一个指针                                     │
│                                                                         │
│   3. 实现极简                                                           │
│      • 核心只需 lowbit 一个操作                                        │
│      • 代码不超过 10 行                                                │
│                                                                         │
│   4. 常数因子小                                                         │
│      • 只有简单的位运算和数组/指针访问                                 │
│      • 缓存友好（顺序访问趋势）                                        │
│                                                                         │
│   这就是为什么 Bitcoin Core 选择 pskip 而非更复杂的数据结构！          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.3 分叉选择机制

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Bitcoin Core 分叉选择流程                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                      收到新区块                                  │  │
│   └───────────────────────────┬─────────────────────────────────────┘  │
│                               │                                         │
│                               ▼                                         │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │   1. AcceptBlockHeader()                                         │  │
│   │      • 验证区块头                                                │  │
│   │      • 创建 CBlockIndex 并加入 mapBlockIndex                    │  │
│   │      • 计算 nChainWork = parent.nChainWork + GetBlockProof()    │  │
│   └───────────────────────────┬─────────────────────────────────────┘  │
│                               │                                         │
│                               ▼                                         │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │   2. 检查是否为候选链头                                          │  │
│   │      if (pindex->nChainWork > m_chain.Tip()->nChainWork)        │  │
│   │          setBlockIndexCandidates.insert(pindex);                │  │
│   └───────────────────────────┬─────────────────────────────────────┘  │
│                               │                                         │
│                               ▼                                         │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │   3. ActivateBestChain()                                         │  │
│   │      • 从 setBlockIndexCandidates 获取最佳候选                  │  │
│   │      • 如果比当前链更重，执行链切换                             │  │
│   └───────────────────────────┬─────────────────────────────────────┘  │
│                               │                                         │
│           ┌───────────────────┴───────────────────┐                    │
│           ▼                                       ▼                    │
│   ┌───────────────────┐               ┌───────────────────┐           │
│   │ 新链更重：切换链  │               │ 当前链更重：忽略  │           │
│   │ DisconnectTip()   │               │ 保留候选以备后用  │           │
│   │ ConnectTip()      │               │                   │           │
│   └───────────────────┘               └───────────────────┘           │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.4 setBlockIndexCandidates 候选集

```cpp
// 候选链头比较器
struct CBlockIndexWorkComparator {
    bool operator()(const CBlockIndex *pa, const CBlockIndex *pb) const {
        // 首先按累积工作量排序（降序）
        if (pa->nChainWork > pb->nChainWork) return false;
        if (pa->nChainWork < pb->nChainWork) return true;
        
        // 工作量相同时，按哈希排序（确定性打破平局）
        return pa->GetBlockHash() < pb->GetBlockHash();
    }
};

// 候选集合
std::set<CBlockIndex*, CBlockIndexWorkComparator> setBlockIndexCandidates;
```

**候选集维护策略**：

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      setBlockIndexCandidates 示意                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   区块树：                        候选集（按 nChainWork 排序）：       │
│       Genesis                     ┌─────────────────────────┐          │
│          │                        │  E: chainwork = 500     │ ← 最佳  │
│          A                        │  C: chainwork = 400     │          │
│         / \                       │  D: chainwork = 380     │          │
│        B   C                      └─────────────────────────┘          │
│       / \                                                               │
│      D   E  ← 当前 tip                                                 │
│                                                                         │
│   规则：                                                                │
│   • 只有叶子节点（无子节点）才能进入候选集                             │
│   • 有子节点的区块自动从候选集移除                                     │
│   • 获取最佳链头: O(1) - 取集合首元素                                  │
│                                                                         │
│   插入新区块 F (基于 D)：                                               │
│       Genesis                     候选集更新：                          │
│          │                        ┌─────────────────────────┐          │
│          A                        │  E: chainwork = 500     │ ← 仍最佳│
│         / \                       │  F: chainwork = 480     │ ← 新增  │
│        B   C                      │  C: chainwork = 400     │          │
│       / \                         └─────────────────────────┘          │
│      D   E                        D 被移除（有子节点了）               │
│      │                                                                  │
│      F                                                                  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.5 寻找两条链的公共祖先（LCA）

在链切换（reorg）时，需要找到两条链的**最近公共祖先（Lowest Common Ancestor, LCA）**。

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      公共祖先问题                                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   场景：当前链头 A，新链头 B，需要找到它们的公共祖先                    │
│                                                                         │
│                    [LCA]  ← 最近公共祖先                               │
│                   /      \                                              │
│                 ...      ...                                            │
│                 /          \                                            │
│               [A]          [B]                                          │
│           当前链头       新链头                                         │
│                                                                         │
│   用途：                                                                │
│   • 链切换时，需要从 LCA 开始回滚/重放区块                             │
│   • 计算两条链的分叉深度                                               │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**算法 1：朴素方法 O(n)**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      朴素 LCA 算法                                      │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   思路：先对齐高度，再同步向上走                                        │
│                                                                         │
│   def find_lca_naive(a, b):                                            │
│       # 步骤 1: 对齐到相同高度                                         │
│       while a.height > b.height:                                        │
│           a = a.pprev                                                   │
│       while b.height > a.height:                                        │
│           b = b.pprev                                                   │
│                                                                         │
│       # 步骤 2: 同步向上直到相遇                                        │
│       while a != b:                                                     │
│           a = a.pprev                                                   │
│           b = b.pprev                                                   │
│                                                                         │
│       return a  # 公共祖先                                             │
│                                                                         │
│   复杂度：O(height_diff + distance_to_lca)                             │
│   最坏情况：O(n)，n 是链长                                             │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**算法 2：利用 pskip 优化 O(log n)**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      pskip 优化的 LCA 算法                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   思路：用 GetAncestor 快速对齐，然后用二分思想快速逼近                 │
│                                                                         │
│   def find_lca_optimized(a, b):                                        │
│       # 步骤 1: 用 GetAncestor 快速对齐高度 O(log n)                   │
│       if a.height > b.height:                                          │
│           a = a.GetAncestor(b.height)                                  │
│       elif b.height > a.height:                                        │
│           b = b.GetAncestor(a.height)                                  │
│                                                                         │
│       # 此时 a 和 b 高度相同                                           │
│       if a == b:                                                        │
│           return a  # 恰好是同一节点                                   │
│                                                                         │
│       # 步骤 2: 二分逼近公共祖先                                        │
│       # 使用 pskip 尝试大步跳跃                                        │
│       while a.pprev != b.pprev:                                        │
│           if a.pskip and a.pskip != b.pskip:                           │
│               # pskip 指向的祖先不同，可以跳                           │
│               a = a.pskip                                              │
│               b = b.pskip                                              │
│           else:                                                         │
│               # pskip 相同或不存在，小步走                             │
│               a = a.pprev                                              │
│               b = b.pprev                                              │
│                                                                         │
│       return a.pprev  # 父节点就是 LCA                                 │
│                                                                         │
│   复杂度：O(log n)                                                      │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**图解：pskip LCA 查找过程**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      示例：找 Block 15 和 Block 14 的分叉链的 LCA      │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   假设区块树结构：                                                      │
│                                                                         │
│   高度:  0    1    2    3    4    5    6    7    8    9   10   11      │
│                                                                         │
│   主链: [0]─[1]─[2]─[3]─[4]─[5]─[6]─[7]─[8]─[9]─[10]─[11]              │
│                             │                                           │
│   分叉:                     └─[5']─[6']─[7']─[8']─[9']                 │
│                                                                         │
│   任务：找 [11] 和 [9'] 的 LCA（应该是 [4]）                           │
│                                                                         │
│   ═══════════════════════════════════════════════════════════════════  │
│                                                                         │
│   步骤 1: 对齐高度                                                      │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  a = [11], height = 11                                          │  │
│   │  b = [9'], height = 9                                           │  │
│   │                                                                  │  │
│   │  a.height > b.height，需要把 a 降到高度 9                       │  │
│   │  a = [11].GetAncestor(9)                                        │  │
│   │                                                                  │  │
│   │  GetAncestor 过程:                                               │  │
│   │    11 → 10 → 9  (用 pskip 加速)                                 │  │
│   │  a = [9]                                                         │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   步骤 2: 同步向上找 LCA                                                │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  现在: a = [9], b = [9']，高度都是 9                            │  │
│   │  a ≠ b，需要继续向上                                            │  │
│   │                                                                  │  │
│   │  检查 pskip:                                                     │  │
│   │    a.pskip = [8], b.pskip = [8']                                │  │
│   │    不同！可以跳                                                  │  │
│   │    a = [8], b = [8']                                            │  │
│   │                                                                  │  │
│   │  继续检查 pskip:                                                 │  │
│   │    a.pskip = [0], b.pskip = [0]（假设 8 和 8' 的 pskip 都指向0）│  │
│   │    相同！不能跳，改用 pprev                                      │  │
│   │    a = [7], b = [7']                                            │  │
│   │                                                                  │  │
│   │  继续 pprev:                                                     │  │
│   │    a = [6], b = [6']                                            │  │
│   │    a = [5], b = [5']                                            │  │
│   │                                                                  │  │
│   │  检查 a.pprev 和 b.pprev:                                       │  │
│   │    a.pprev = [4], b.pprev = [4]                                 │  │
│   │    相同！找到 LCA                                                │  │
│   │                                                                  │  │
│   │  返回 [4]                                                        │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**完整实现**

```python
class BlockIndex:
    def __init__(self, height, block_hash, pprev=None):
        self.height = height
        self.hash = block_hash
        self.pprev = pprev
        self.pskip = None
    
    def get_ancestor(self, target_height):
        """O(log n) 获取指定高度的祖先"""
        if target_height > self.height or target_height < 0:
            return None
        
        current = self
        while current.height > target_height:
            skip_height = current.height - lowbit(current.height)
            if current.pskip and skip_height >= target_height:
                current = current.pskip
            else:
                current = current.pprev
        return current


def find_lca(a: BlockIndex, b: BlockIndex) -> BlockIndex:
    """
    找到两个区块的最近公共祖先 (LCA)
    复杂度: O(log n)
    """
    # 步骤 1: 对齐到相同高度
    if a.height > b.height:
        a = a.get_ancestor(b.height)
    elif b.height > a.height:
        b = b.get_ancestor(a.height)
    
    # 如果对齐后就相同，直接返回
    if a == b:
        return a
    
    # 步骤 2: 用 pskip 加速向上查找
    while a != b:
        # 如果 pskip 指向不同的祖先，可以大步跳
        if a.pskip and b.pskip and a.pskip != b.pskip:
            a = a.pskip
            b = b.pskip
        else:
            # 否则小步走
            a = a.pprev
            b = b.pprev
            
        # 安全检查
        if a is None or b is None:
            return None
    
    return a


def find_lca_with_path(a: BlockIndex, b: BlockIndex):
    """
    找到 LCA 并返回需要回滚/重放的路径
    返回: (lca, path_from_a, path_from_b)
    """
    path_a = []
    path_b = []
    
    # 记录路径同时对齐高度
    while a.height > b.height:
        path_a.append(a)
        a = a.pprev
    while b.height > a.height:
        path_b.append(b)
        b = b.pprev
    
    # 同步向上找 LCA
    while a != b:
        path_a.append(a)
        path_b.append(b)
        a = a.pprev
        b = b.pprev
    
    lca = a
    return lca, path_a, path_b


# 使用示例
def reorg(current_tip, new_tip):
    """执行链切换"""
    lca, disconnect_path, connect_path = find_lca_with_path(current_tip, new_tip)
    
    print(f"LCA at height {lca.height}")
    print(f"Need to disconnect {len(disconnect_path)} blocks")
    print(f"Need to connect {len(connect_path)} blocks")
    
    # 回滚: 从 current_tip 退回到 LCA
    for block in disconnect_path:
        disconnect_block(block)
    
    # 重放: 从 LCA 前进到 new_tip (需要反转路径)
    for block in reversed(connect_path):
        connect_block(block)
```

**算法复杂度分析**

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      LCA 算法复杂度对比                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   算法                    │  高度对齐      │  找 LCA        │  总复杂度 │
│   ────────────────────────┼───────────────┼───────────────┼──────────  │
│   朴素 (只用 pprev)       │  O(Δh)        │  O(d)         │  O(n)     │
│   pskip 优化              │  O(log Δh)    │  O(log d)     │  O(log n) │
│                                                                         │
│   其中:                                                                 │
│   • Δh = 两个节点的高度差                                              │
│   • d = 从高度对齐点到 LCA 的距离                                      │
│   • n = 链的总长度                                                     │
│                                                                         │
│   ───────────────────────────────────────────────────────────────────  │
│                                                                         │
│   为什么 pskip 能加速找 LCA？                                          │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   pskip 的跳跃距离是 2 的幂次（1, 2, 4, 8, ...）                │  │
│   │                                                                  │  │
│   │   当 a.pskip ≠ b.pskip 时，说明 LCA 在它们的公共祖先之下         │  │
│   │   可以安全地同时跳过 lowbit 个节点                               │  │
│   │                                                                  │  │
│   │   这类似于二分查找的思想：                                       │  │
│   │   每次要么跳过一大段，要么排除一大段                             │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**更精确的 LCA 算法（Binary Lifting）**

```python
def find_lca_binary_lifting(a: BlockIndex, b: BlockIndex) -> BlockIndex:
    """
    使用 Binary Lifting 技术的 LCA 算法
    需要预处理 ancestors[i][j] = i 的第 2^j 个祖先
    这里用 pskip 近似实现
    """
    # 对齐高度
    if a.height < b.height:
        a, b = b, a  # 确保 a 更高
    
    # 用 pskip 快速降低 a 的高度
    a = a.get_ancestor(b.height)
    
    if a == b:
        return a
    
    # 二分提升：从大步到小步尝试
    # 如果跳过后仍然不同，就跳
    # 如果跳过后相同，就不跳（LCA 在中间）
    while a.pprev != b.pprev:
        if a.pskip and b.pskip and a.pskip != b.pskip:
            a = a.pskip
            b = b.pskip
        elif a.pprev and b.pprev:
            a = a.pprev
            b = b.pprev
        else:
            break
    
    return a.pprev
```

### 7.6 链切换（Reorg）机制

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Bitcoin Core 链切换流程                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   场景：当前链 A→B→C→D，发现更长链 A→B→E→F→G                          │
│                                                                         │
│          A ← B ← C ← D  (当前链, chainwork=400)                        │
│               └── E ← F ← G  (更长链, chainwork=450)                   │
│                                                                         │
│   步骤 1: FindForkPoint()                                               │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  • 从两个链头向前遍历找公共祖先                                  │  │
│   │  • 使用 pskip 加速: O(log n)                                    │  │
│   │  • 结果: 公共祖先 = B                                            │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   步骤 2: DisconnectTip() × 2                                           │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  • 回滚 D: 撤销交易，更新 UTXO                                  │  │
│   │  • 回滚 C: 撤销交易，更新 UTXO                                  │  │
│   │  • 被回滚的交易返回 mempool                                      │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   步骤 3: ConnectTip() × 3                                              │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  • 连接 E: 验证并应用交易                                       │  │
│   │  • 连接 F: 验证并应用交易                                       │  │
│   │  • 连接 G: 验证并应用交易                                       │  │
│   │  • 更新 m_chain 指向新链                                        │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   结果：                                                                │
│   • m_chain: [Genesis, A, B, E, F, G]                                  │
│   • C, D 变成孤立分支（但仍保留在 mapBlockIndex）                      │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.6 Bitcoin Core vs Proto-Array 对比

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      核心设计对比                                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   特性                │  Bitcoin Core          │  Proto-Array           │
│   ═══════════════════╪═══════════════════════╪═══════════════════════  │
│   存储结构            │  HashMap + 指针链      │  数组 + 索引            │
│   区块索引            │  哈希查找 O(1)*        │  数组索引 O(1)          │
│   父节点访问          │  指针解引用            │  数组索引               │
│   祖先查找            │  pskip O(log n)        │  遍历 O(depth)          │
│   最佳链头            │  候选集首元素 O(1)     │  best_descendant O(1)   │
│   权重更新            │  仅新区块 O(1)         │  增量传播 O(depth)      │
│   内存管理            │  保留所有区块索引      │  剪枝已确定区块         │
│   缓存友好性          │  较差（指针跳跃）      │  较好（连续数组）       │
│                                                                         │
│   * HashMap 有哈希碰撞开销                                              │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      设计哲学对比                                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   Bitcoin Core 的选择：                                                 │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   1. 简单直接                                                    │  │
│   │      • 分叉罕见（~10分钟一个块）                                │  │
│   │      • 不需要复杂优化                                           │  │
│   │                                                                  │  │
│   │   2. 候选集方案                                                  │  │
│   │      • setBlockIndexCandidates 只存叶子节点                     │  │
│   │      • 通常只有 1-3 个候选                                      │  │
│   │      • 获取最佳: O(1)                                           │  │
│   │                                                                  │  │
│   │   3. 保留完整历史                                                │  │
│   │      • 所有区块索引永久保留                                     │  │
│   │      • 便于查询任意历史区块                                     │  │
│   │      • 支持深度重组（虽然极少发生）                             │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   Proto-Array 的选择：                                                  │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │   1. 高频更新优化                                                │  │
│   │      • 每秒数千条证明                                           │  │
│   │      • 需要增量权重传播                                         │  │
│   │                                                                  │  │
│   │   2. 预计算 best_descendant                                      │  │
│   │      • 避免每次遍历树                                           │  │
│   │      • 分叉选择 O(1)                                            │  │
│   │                                                                  │  │
│   │   3. 积极剪枝                                                    │  │
│   │      • finalized 之前的区块丢弃                                 │  │
│   │      • 节约内存，提高缓存命中                                   │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.7 为什么比特币不需要 Proto-Array？

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      比特币不需要 Proto-Array 的原因                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   1. 更新频率低                                                         │
│   ═══════════════                                                      │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  比特币：~10分钟一个区块                                        │  │
│   │  以太坊：每秒数千条证明 + 12秒一个区块                          │  │
│   │                                                                  │  │
│   │  比特币的分叉选择频率：~0.0017 次/秒                            │  │
│   │  以太坊的分叉选择频率：~1000+ 次/秒                             │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   2. 无投票权重                                                         │
│   ═══════════════                                                      │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  比特币：区块工作量在创建时确定，之后不变                       │  │
│   │  以太坊：区块权重随证明累积动态变化                             │  │
│   │                                                                  │  │
│   │  Proto-Array 的增量更新机制主要解决的就是权重动态变化问题       │  │
│   │  比特币没有这个问题                                              │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   3. 候选集足够高效                                                     │
│   ═══════════════════                                                  │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  setBlockIndexCandidates 通常只有 1-3 个元素                    │  │
│   │  std::set 插入/查找: O(log n) ≈ O(1)                            │  │
│   │                                                                  │  │
│   │  对于比特币的工作负载，这已经足够快                             │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   4. 历史查询需求                                                       │
│   ═══════════════                                                      │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  比特币全节点需要响应任意历史区块查询                           │  │
│   │  保留完整 mapBlockIndex 是有意义的                              │  │
│   │                                                                  │  │
│   │  以太坊 PoS 可以依赖执行层存储历史                              │  │
│   │  共识层只需关注最近的未确定区块                                 │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.8 如果要优化比特币分叉处理

虽然 Bitcoin Core 当前方案已经足够，但如果需要进一步优化（例如处理大量分叉攻击），可以借鉴 Proto-Array 思想：

```python
class HybridBitcoinForkChoice:
    """
    混合方案：结合 Bitcoin Core 和 Proto-Array 优点
    适用于需要处理大量分叉的场景
    """
    
    def __init__(self):
        # Bitcoin Core 风格：候选集
        self.candidates = SortedSet(key=lambda x: -x.chainwork)
        
        # Proto-Array 风格：数组存储 + 索引
        self.nodes = []  # 只存储近期区块
        self.indices = {}
        
        # 确认深度（类似 finalized）
        self.confirmed_depth = 6
    
    def on_block(self, block):
        # 1. 添加到数组
        node = self._create_node(block)
        self.nodes.append(node)
        self.indices[block.hash] = len(self.nodes) - 1
        
        # 2. 更新候选集（只保留叶子）
        parent_index = self.indices.get(block.parent_hash)
        if parent_index is not None:
            parent = self.nodes[parent_index]
            if parent in self.candidates:
                self.candidates.remove(parent)
        
        self.candidates.add(node)
        
        # 3. 剪枝已确认区块
        self._maybe_prune()
    
    def get_best_tip(self):
        # O(1) 获取最佳链头
        return self.candidates[0] if self.candidates else None
    
    def _maybe_prune(self):
        # 类似 Proto-Array 的剪枝
        tip = self.get_best_tip()
        if tip and tip.height > self.confirmed_depth:
            confirm_height = tip.height - self.confirmed_depth
            # 移除 confirm_height 之前的区块...
```

---

## 8. 性能分析与对比

### 8.1 复杂度对比

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      复杂度对比                                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   操作              │  朴素实现              │  Proto-Array             │
│   ─────────────────┼───────────────────────┼─────────────────────────  │
│   添加新区块        │  O(1)                 │  O(depth)                │
│   获取链头          │  O(nodes)             │  O(1)                    │
│   处理投票/证明     │  O(nodes × validators)│  O(depth)                │
│   内存查找          │  HashMap O(1)*        │  Array O(1)              │
│   链切换判断        │  O(depth)             │  O(1)                    │
│                                                                         │
│   * HashMap 有哈希碰撞和缓存不友好的问题                                │
│                                                                         │
│   以太坊场景（100万验证者，正常 ~100 节点）：                           │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  朴素: 每次分叉选择 ~O(100万) 次权重计算                        │  │
│   │  Proto-Array: ~O(100) 次数组访问（2-3 epochs 深度）             │  │
│   │  提升: ~10000 倍                                                │  │
│   │                                                                  │  │
│   │  注：Checkpoint 机制限制了需要维护的区块数量                    │  │
│   │  • finalized 每 2 epochs 推进                                   │  │
│   │  • 正常情况只需维护 64-96 个 slots 的区块                       │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   比特币场景（偶发分叉，~6 节点深度）：                                 │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  朴素: 遍历分叉找最长链 O(forks)                                │  │
│   │  Proto-Array: O(1) 直接获取                                     │  │
│   │  提升: 在大量分叉时显著                                         │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 8.2 内存优化

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      内存占用分析                                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   单个 ProtoNode 大小：                                                 │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  root: 32 bytes                                                 │  │
│   │  parent: 8 bytes                                                │  │
│   │  slot/height: 8 bytes                                           │  │
│   │  weight/chainwork: 8-32 bytes                                   │  │
│   │  best_child: 8 bytes                                            │  │
│   │  best_descendant: 8 bytes                                       │  │
│   │  其他元数据: ~16 bytes                                          │  │
│   │  ─────────────────────────                                      │  │
│   │  合计: ~100 bytes/node                                          │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   以太坊 Proto-Array 节点数分析：                                       │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │                                                                  │  │
│   │  Checkpoint 机制：                                               │  │
│   │  • 1 epoch = 32 slots（每 slot 12 秒）                          │  │
│   │  • Finalization 通常需要 2 epochs                               │  │
│   │  • Proto-Array 只维护 finalized_checkpoint 之后的区块           │  │
│   │                                                                  │  │
│   │  正常情况节点数：                                                │  │
│   │  • 从 finalized 到 head ≈ 2-3 epochs                            │  │
│   │  • 约 64-96 slots                                               │  │
│   │  • 考虑偶发分叉：~100 个节点                                    │  │
│   │                                                                  │  │
│   │  异常情况（网络分区、finalization 延迟）：                       │  │
│   │  • 可能积累更多未 finalized 区块                                │  │
│   │  • 极端情况可能 200+ 节点                                       │  │
│   │                                                                  │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   典型场景内存占用：                                                    │
│   ┌─────────────────────────────────────────────────────────────────┐  │
│   │  以太坊（正常 ~100 节点）：                                      │  │
│   │  100 × 100 bytes = 10 KB                                        │  │
│   │                                                                  │  │
│   │  比特币（~10 分叉节点）：                                        │  │
│   │  10 × 100 bytes = 1 KB                                          │  │
│   │                                                                  │  │
│   │  对比存储整条链：                                                │  │
│   │  以太坊: ~20M 区块 × 100 bytes = 2 GB                           │  │
│   │  比特币: ~800K 区块 × 100 bytes = 80 MB                         │  │
│   └─────────────────────────────────────────────────────────────────┘  │
│                                                                         │
│   通过只维护 finalized/confirmed 之后的节点，内存减少 99.99%+          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 8.3 实际性能测试

```python
import time

def benchmark_proto_array():
    """性能基准测试"""
    
    # 构建大型区块树
    pa = BitcoinProtoArray()
    genesis = b'genesis'
    pa.on_block(genesis, b'', height=0, difficulty=1)
    
    # 添加 10000 个区块
    start = time.time()
    parent = genesis
    for i in range(1, 10001):
        block = sha256(f"block_{i}".encode()).digest()
        pa.on_block(block, parent, height=i, difficulty=1000)
        parent = block
    add_time = time.time() - start
    print(f"添加 10000 区块: {add_time:.3f}s ({10000/add_time:.0f} blocks/s)")
    
    # 获取链头 10000 次
    start = time.time()
    for _ in range(10000):
        _ = pa.get_best_chain_tip()
    get_time = time.time() - start
    print(f"获取链头 10000 次: {get_time:.6f}s ({10000/get_time:.0f} ops/s)")
    
    # 添加 100 个分叉
    start = time.time()
    for i in range(100):
        fork_parent = sha256(f"block_{5000+i}".encode()).digest()
        for j in range(10):
            fork = sha256(f"fork_{i}_{j}".encode()).digest()
            pa.on_block(fork, fork_parent, height=5001+i+j, difficulty=1000)
            fork_parent = fork
    fork_time = time.time() - start
    print(f"添加 100 个分叉(每个10块): {fork_time:.3f}s")
    
    print(f"\n总节点数: {len(pa.nodes)}")
    print(f"内存占用估计: {len(pa.nodes) * 100 / 1024:.1f} KB")

benchmark_proto_array()
```

---

## 9. 总结

### 9.1 核心要点

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Proto-Array 核心要点                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   1. 数据结构创新                                                       │
│   ═══════════════                                                      │
│   • 树展平为数组，索引替代哈希                                         │
│   • 预计算 best_child 和 best_descendant                               │
│   • 只维护未确定的区块，定期剪枝                                       │
│                                                                         │
│   2. 算法优化                                                           │
│   ═══════════                                                          │
│   • 增量更新：权重变化沿路径传播                                       │
│   • O(1) 分叉选择：直接返回预计算结果                                  │
│   • 批量处理：聚合多个更新后一次性传播                                 │
│                                                                         │
│   3. 通用性                                                             │
│   ═══════                                                              │
│   • 可应用于任何需要分叉选择的区块链                                   │
│   • 以太坊：权重 = 验证者投票累积                                      │
│   • 比特币：权重 = 工作量（难度）累积                                  │
│   • 其他：可自定义权重计算规则                                         │
│                                                                         │
│   4. 工程实践                                                           │
│   ═══════════                                                          │
│   • Lighthouse、Prysm、Teku 等主流客户端均使用                         │
│   • 经过百万验证者规模验证                                             │
│   • 开源实现可直接复用                                                 │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 9.2 适用场景

| 场景 | 是否适用 | 说明 |
|-----|---------|------|
| 以太坊 PoS | ✅ 最佳 | 高频投票更新，100万验证者 |
| 比特币 PoW | ✅ 适用 | 分叉处理，最长链选择 |
| Tendermint BFT | ⚠️ 一般 | 很少分叉，简单实现即可 |
| DAG 结构 | ❌ 不适用 | 非树结构，需要其他方案 |

### 9.3 扩展阅读

1. **Lighthouse 实现**
   - https://github.com/sigp/lighthouse/tree/stable/consensus/proto_array

2. **Prysm 实现**
   - https://github.com/prysmaticlabs/prysm/tree/develop/beacon-chain/forkchoice/protoarray

3. **以太坊共识规范**
   - https://github.com/ethereum/consensus-specs/blob/dev/specs/phase0/fork-choice.md

---

*文档版本: 1.0*
*最后更新: 2025年*

