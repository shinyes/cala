# P5 统计 验收证据

- Date: `2026-09-10`
- Plan: [`docs/aegis/plans/2026-09-10-p5-stats.md`](../../../plans/2026-09-10-p5-stats.md)
- 范围：规格 §16 的 **P5 统计**
- 出口条件：**A8**（8 项指标含中位数在奇偶长度下正确）+ 功能 4

---

## 1. 全量命令结果

| 命令 | 结果 |
|---|---|
| `cd backend && go build ./...` | exit 0 |
| `cd backend && go vet ./...` | exit 0 |
| `cd backend && gofmt -l internal cmd` | 无输出 |
| `cd backend && go test ./... -count=1` | **9 个包全部 ok**（新增 internal/stats） |
| `cd app && flutter analyze` | `No issues found!` |
| `cd app && flutter test` | **107 项全通过** |

新增测试：`internal/stats` 36 项、`internal/api` 统计端点 20 项、折线图 13 项。

---

## 2. 验收标准 A8

> A8 统计 8 个指标（含中位数，奇偶长度）计算结果正确

### 2.1 中位数（最易错处）

`TestMedian` 10 个子用例 PASS：空 / 单元素 / 奇数三、五个 / **偶数两、四、六个** /
未排序输入 / 含重复值 / 浮点值。

`TestMedianEvenIsAverageNotLowerMiddle` 专门区分「取中间两个的平均」与「取下中位数」——
这是最容易被写错、且错了以后数字看起来仍然合理的地方。

### 2.2 8 项指标

`TestComputeMetricsEmpty` / `Single` / `OddCount` / `EvenCount` / `UnaffectedByInputOrder` /
`ZeroTimeRound` 全部 PASS。

偶数样本量的核对（`TestComputeMetricsEvenCount`）：

```
耗时 1000, 4000, 2000, 3000  -> min 1000, max 4000, avg 2500, median 2500
正确率 1.0, 0.25, 0.5, 0.75  -> min 0.25, max 1.0, avg 0.625, median 0.625
```

### 2.3 「平均正确率」的语义

`TestAccuracyAverageIsMeanOfRoundAccuracies` 用**题数不同**的两轮固定语义：

```
1/1 题（正确率 1.0）与 0/9 题（正确率 0.0）
  各轮正确率的平均 = (1.0 + 0.0) / 2 = 0.5   <- 规格 §8.1 要求的值
  总正确数/总题数  = 1/10           = 0.1   <- 实现必须不是这个
```

两者在每轮题数相同时相等，题数不同时不同。测试断言取前者并显式排除后者。

---

## 3. 分桶正确性

`internal/stats` 分桶测试 11 项 PASS，覆盖：

| 用例 | 断言 |
|---|---|
| `TestGroupDay` | 同自然日聚合；不同日分开；顺序为时间升序 |
| `TestGroupDayRespectsTimezone` | **时区偏移**：UTC 16:30 在东八区属于**次日** |
| `TestGroupDayAcrossYearBoundary` | 跨年顺序正确（不依赖字符串比较的巧合） |
| `TestGroupWeek` | 不同日期的同一星期几聚合到一起 |
| `TestGroupWeekStartsOnMonday` | 顺序为周一→周日，与传入顺序无关 |
| `TestGroupWeekOmitsEmptyDays` | 只返回有数据的桶 |
| `TestGroupMonth` / `AcrossYearBoundary` | 跨月、跨年顺序 |
| `TestGroupEmptyInput` | 返回空切片而非 nil |

**时区那条最关键**：若按 UTC 分桶，用户在当地 00:30 做的练习会被算到前一天，
「今天的练习」在他自己看来就是错的。端点因此接受 `tzOffsetMinutes`。

---

## 4. 端到端验证（真实进程 / HTTP / SQLite）

```
项目 id=1
第 1 轮: 正确 4/4  每题耗时 1000ms
第 2 轮: 正确 2/4  每题耗时 2000ms
第 3 轮: 正确 0/4  每题耗时 3000ms

=== 统计（三种粒度）===
  [day]   轮数=3  耗时 min/max/avg/median = 4000/12000/8000/8000
                  正确率 min/max/avg/median = 0/1/0.5/0.5
  [week]  同上，桶数=1  标签=周五
  [month] 同上，桶数=1  标签=2026年9月
```

**手工核对**：三轮总耗时 4000 / 8000 / 12000 →
min = 4000 ✅、max = 12000 ✅、avg = 8000 ✅、median = 8000 ✅；
三轮正确率 1.0 / 0.5 / 0.0 → min = 0 ✅、max = 1 ✅、avg = 0.5 ✅、median = 0.5 ✅。

**UTF-8 编码确认**（避免把控制台显示问题误判为数据问题）：
直接按原始字节读取响应，严格 UTF-8 解码通过，`label` 的码点为
`U+5468 周`、`U+4E94 五` —— 数据本身正确，此前控制台显示为乱码只是
PowerShell 5.1 的输出编码所致。

---

## 5. 变异测试取证

把中位数的偶数分支改为「只取下中位数」（`xs[n/2-1]`）后：

```
metrics_test.go:36: median([1 3]) = 1, 期望 2
metrics_test.go:36: median([1 2 3 4]) = 2, 期望 2.5
metrics_test.go:36: median([10 20 30 40 50 60]) = 30, 期望 35
metrics_test.go:36: median([4 1 3 2]) = 2, 期望 2.5
metrics_test.go:36: median([0.1 0.2 0.3 0.4]) = 0.2, 期望 0.25
--- FAIL: TestMedian
    --- FAIL: TestMedian/偶数两个 / 偶数四个 / 偶数六个 / 未排序输入 / 浮点值
    --- PASS: TestMedian/空 / 单元素 / 奇数三个 / 奇数五个 / 含重复值
```

**注意奇数用例仍然 PASS**：这说明测试确实在区分奇偶分支，
而不是「一改就全红」的粗放断言。已还原（`grep MUTATION` 无残留）。

---

## 6. 隐私边界

`TestStatsIsPrivate` PASS：作者与订阅者在同一共享项目上各自做练习后，
各自的统计只包含**自己**的轮次（作者 2 轮、订阅者 3 轮，互不可见）。

这不是权限细节而是数据所有权：答题记录属于做题的人，统计由记录派生，同样属于个人。

实现方式是取数时按 `(userID, projectID)` 过滤——**没有**任何「按项目汇总所有用户」
的查询路径，因此不存在遗漏授权的可能。

---

## 7. 「统计是派生态」的验证

`TestStatsDisappearWithProject` PASS：做 3 轮 → 统计显示 3 轮 → 删除项目 →
统计端点返回 404（项目已不存在）。

由于没有物化聚合表，统计随 `practice_round` 的级联删除自然消失，
**不存在「记录删了但聚合还在」的可能**——这是规格 §8.1 与基线 §5.2(4) 的直接结果。

---

## 8. 端点契约

| 参数 | 校验 | 测试 |
|---|---|---|
| `grain` | `day`/`week`/`month`，缺省 `day`，非法 -> 400 | `TestStatsGrainValidation` |
| `tzOffsetMinutes` | 整数且 ∈ [-840, 840]，越界/非整数 -> 400 | `TestStatsTZOffsetValidation` |

其他端点行为（`TestStatsRequiresAuthAndAccess`、`TestStatsEmptyProject`）：
未登录 401、无关用户 403、项目不存在 404、无记录时 `roundCount=0` 且
`buckets` 为**空数组而非 null**（客户端无需额外判空）。

---

## 9. 前端

- `stats_tab.dart` 替换 P4 的占位，按规格 §8.2 的信息架构实现：
  项目选择器 / 粒度分段控件 / 指标分段控件 / 单条折线 / 两张指标卡各 4 行
- **本页只读**：代码中无任何写操作调用（无新建、无编辑、无删除）
- `line_chart.dart` 用 `CustomPainter` 自绘，**零新增依赖**
- 折线图边界测试 13 项：空数据、单点、多点、全等值、60 点、极小尺寸、
  `valueFormatter` 调用次数
- `yRange` 的除零防护单独测试：空输入、单元素、全等值、全为 0 都给出非零跨度

---

## 10. 关于浮点的边界（记录以免后续误判）

规格 §5.5.3 禁止**判分**路径出现浮点，理由是跨端精确性（Go 与 Dart 逐例一致）。
统计**不满足**该前提：它只在服务端计算，客户端仅渲染返回值，没有跨端契约。

因此统计使用 `float64`。这一区分已写入 `internal/stats/bucket.go` 的包注释，
避免后来者以为「本项目禁止浮点」而在统计里硬造有理数。

---

## 11. 未验证项

| # | 项 | 原因 | 何时可验 |
|---|---|---|---|
| 1 | 统计页在真机/浏览器上的人工浏览 | 需要交互会话；已用端到端 HTTP + 折线图 widget 测试替代 | 用户可 `flutter run -d chrome` |
| 2 | 跨时区的真实用户行为 | 已用 `tzOffsetMinutes` 参数测试三种偏移 | 用户在不同时区使用时 |
| 3 | 大量历史数据的图表可读性（数百个桶） | 个人练习数据量下未构成问题；已测 60 点抽稀路径 | 若用户积累多年数据 |
| 4 | CI 首跑结果 | 本环境不可达 github.com | 需用户在 Actions 页确认 |

---

## 12. 结论

规格 §16 的 **P5 出口条件已满足**：A8 通过（且有变异测试取证），
功能 4 交付，隐私边界与「统计为派生态」均有测试守护。

**新增规模**：`internal/stats`（4 文件，其中 2 个测试）、
`service/stats.go`、`api/stats.go`；前端 4 文件。
零新增第三方依赖。

**下一步**：P6 分享订阅与退订（分享 token、多链接导入、退订清历史）。
