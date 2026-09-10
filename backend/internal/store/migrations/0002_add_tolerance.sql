-- 规格 §6 的 Design Defect 修正：D3 承诺「项目级可选容差」，但 0001 未建字段。
--
-- 两列同时为空        = 精确比较
-- 两列同时非空且 > 0  = 容差 num/den
--
-- 用两个整数而非浮点，使容差本身也留在无浮点判分路径上（规格 §5.5.3）。
-- 「必须成对」这一不变式由 service 层校验（SQLite 的 ADD COLUMN 不便追加
-- 引用其他列的 CHECK 约束），并有对应测试守护。
ALTER TABLE project ADD COLUMN tolerance_num INTEGER;
ALTER TABLE project ADD COLUMN tolerance_den INTEGER;
