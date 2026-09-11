import 'dart:math' as math;

import 'package:flutter/cupertino.dart';

/// 折线图的数据点。
class ChartPoint {
  final String label;
  final double value;
  const ChartPoint(this.label, this.value);
}

/// 自绘单条折线图。
///
/// 为什么自绘而不引图表库：P4 已明确拒绝图表库（一个依赖换一条折线不值得），
/// 且 Cupertino 本身没有图表组件。
///
/// 设计约束（规格 §8.2）：
///  - **只画一条线**，绝不双 Y 轴。8 个指标分在两张卡里，不混进同一张图。
///  - x 轴按桶顺序等距分布，不按时间比例——周的粒度下时间轴没有意义
///    （周三与周五之间不是「两天」，而是两个分类）。
class LineChart extends StatelessWidget {
  const LineChart({
    super.key,
    required this.points,
    required this.color,
    this.valueFormatter,
    this.height = 180,
  });

  final List<ChartPoint> points;
  final Color color;

  /// 把值渲染成 y 轴标签（例如把毫秒渲染为秒、把比例渲染为百分比）。
  final String Function(double)? valueFormatter;

  final double height;

  @override
  Widget build(BuildContext context) {
    if (points.isEmpty) {
      return SizedBox(
        height: height,
        child: const Center(
          child: Text(
            '还没有可绘制的数据',
            style: TextStyle(fontSize: 13, color: CupertinoColors.tertiaryLabel),
          ),
        ),
      );
    }

    return SizedBox(
      height: height,
      width: double.infinity,
      child: CustomPaint(
        painter: LineChartPainter(
          points: points,
          color: color,
          gridColor: CupertinoColors.tertiarySystemFill.resolveFrom(context),
          labelColor: CupertinoColors.secondaryLabel.resolveFrom(context),
          valueFormatter: valueFormatter,
        ),
      ),
    );
  }
}

/// 折线图的绘制器。
///
/// 单独成类（而非内联到 LineChart）以便直接对绘制逻辑做单元测试：
/// 边界处理（空数据、单点、全等值）都在这里，是最容易除零的地方。
class LineChartPainter extends CustomPainter {
  LineChartPainter({
    required this.points,
    required this.color,
    required this.gridColor,
    required this.labelColor,
    this.valueFormatter,
  });

  final List<ChartPoint> points;
  final Color color;
  final Color gridColor;
  final Color labelColor;
  final String Function(double)? valueFormatter;

  /// y 轴只在底部与顶部保留的边距比例。
  static const _topPadding = 18.0;
  static const _bottomPadding = 26.0;
  static const _leftPadding = 52.0;
  static const _rightPadding = 12.0;

  /// 计算 y 轴范围。
  ///
  /// 返回 (min, max)。所有值相同时给一个非零跨度，
  /// 否则 span 为 0 会导致除零、整条线画到 NaN 位置。
  static (double, double) yRange(List<double> values) {
    if (values.isEmpty) return (0, 1);

    var lo = values.first;
    var hi = values.first;
    for (final v in values) {
      if (v < lo) lo = v;
      if (v > hi) hi = v;
    }

    if (hi == lo) {
      // 全等值：给一个围绕该值的固定跨度，使线画在中间
      final pad = hi.abs() * 0.1 + 1;
      return (lo - pad, hi + pad);
    }

    // 上下各留 10% 余量，避免线贴着边框
    final pad = (hi - lo) * 0.1;
    return (lo - pad, hi + pad);
  }

  @override
  void paint(Canvas canvas, Size size) {
    if (points.isEmpty) return;

    final plotLeft = _leftPadding;
    final plotRight = size.width - _rightPadding;
    final plotTop = _topPadding;
    final plotBottom = size.height - _bottomPadding;

    // 尺寸过小时直接放弃绘制，而不是画出错乱的图形
    if (plotRight <= plotLeft || plotBottom <= plotTop) return;

    final values = points.map((p) => p.value).toList(growable: false);
    final (lo, hi) = yRange(values);
    final span = hi - lo;
    if (span <= 0 || span.isNaN) return;

    double yFor(double v) =>
        plotBottom - (v - lo) / span * (plotBottom - plotTop);

    // x 按索引等距分布（不是按时间比例）
    double xFor(int i) {
      if (points.length == 1) return (plotLeft + plotRight) / 2;
      return plotLeft +
          (plotRight - plotLeft) * (i / (points.length - 1));
    }

    _paintGrid(canvas, plotLeft, plotRight, plotTop, plotBottom, lo, hi, yFor);
    _paintLine(canvas, xFor, yFor);
    _paintXLabels(canvas, xFor, plotBottom, size);
  }

  void _paintGrid(
    Canvas canvas,
    double left,
    double right,
    double top,
    double bottom,
    double lo,
    double hi,
    double Function(double) yFor,
  ) {
    final paint = Paint()
      ..color = gridColor
      ..strokeWidth = 1;

    // 三条横线：底、中、顶；并标注数值
    for (final v in [lo, (lo + hi) / 2, hi]) {
      final y = yFor(v);
      canvas.drawLine(Offset(left, y), Offset(right, y), paint);
      _paintText(
        canvas,
        _format(v),
        Offset(left - 6, y),
        align: TextAlign.right,
        anchorRight: true,
        vCenter: true,
      );
    }
  }

  void _paintLine(Canvas canvas, double Function(int) xFor, double Function(double) yFor) {
    if (points.length == 1) {
      // 单点：画一个点，不画线
      canvas.drawCircle(
        Offset(xFor(0), yFor(points.first.value)),
        4,
        Paint()..color = color,
      );
      return;
    }

    final path = Path();
    for (var i = 0; i < points.length; i++) {
      final o = Offset(xFor(i), yFor(points[i].value));
      if (i == 0) {
        path.moveTo(o.dx, o.dy);
      } else {
        path.lineTo(o.dx, o.dy);
      }
    }

    canvas.drawPath(
      path,
      Paint()
        ..color = color
        ..strokeWidth = 2
        ..style = PaintingStyle.stroke
        ..strokeJoin = StrokeJoin.round
        ..strokeCap = StrokeCap.round,
    );

    // 点太多时省略数据点标记，否则图会变成一团
    if (points.length <= 30) {
      final dot = Paint()..color = color;
      for (var i = 0; i < points.length; i++) {
        canvas.drawCircle(Offset(xFor(i), yFor(points[i].value)), 2.5, dot);
      }
    }
  }

  void _paintXLabels(Canvas canvas, double Function(int) xFor, double plotBottom, Size size) {
    // 标签过密时抽稀：最多显示约 6 个
    final maxLabels = math.max(2, math.min(6, points.length));
    final step = math.max(1, (points.length / maxLabels).ceil());

    for (var i = 0; i < points.length; i += step) {
      _paintText(
        canvas,
        points[i].label,
        Offset(xFor(i), plotBottom + 6),
        align: TextAlign.center,
        hCenter: true,
      );
    }

    // 保证最后一个标签出现（否则末尾的日期看不到）
    final last = points.length - 1;
    if (last > 0 && last % step != 0) {
      final x = xFor(last);
      // 与前一个标签距离过近就跳过，避免文字重叠
      if (x - xFor(last - last % step) > 40) {
        _paintText(
          canvas,
          points[last].label,
          Offset(x, plotBottom + 6),
          align: TextAlign.center,
          hCenter: true,
        );
      }
    }
  }

  String _format(double v) =>
      valueFormatter?.call(v) ?? v.toStringAsFixed(0);

  void _paintText(
    Canvas canvas,
    String text,
    Offset at, {
    TextAlign align = TextAlign.left,
    bool anchorRight = false,
    bool hCenter = false,
    bool vCenter = false,
  }) {
    final tp = TextPainter(
      text: TextSpan(
        text: text,
        style: TextStyle(fontSize: 10, color: labelColor),
      ),
      textAlign: align,
      textDirection: TextDirection.ltr,
    )..layout();

    var dx = at.dx;
    if (anchorRight) dx -= tp.width;
    if (hCenter) dx -= tp.width / 2;
    final dy = vCenter ? at.dy - tp.height / 2 : at.dy;

    tp.paint(canvas, Offset(dx, dy));
  }

  @override
  bool shouldRepaint(LineChartPainter old) =>
      old.points != points || old.color != color;
}
