# 曼德勃罗集渲染技术调研报告

> 调研日期：2026-07-13

---

## 1. 个人桌面主机上，如何提高性能

### 1.1 核心瓶颈分析

曼德勃罗集计算是典型的 **embarrassingly parallel** 问题 — 每个像素独立计算逃逸迭代次数。但由于每个像素迭代次数不同（循环提前退出），编译器**无法自动向量化**内层循环。即使在 i9 (10C/20T) 上，朴素标量实现的浮点吞吐仅达到理论峰值的 ~4%（约 26× 低于理论峰值）。

### 1.2 SIMD 显式向量化（4–8× 加速）

编译器（GCC/Clang）的 `-O3 -ffast-math -march=native` **对 Mandelbrot 内层循环自动向量化失效**，必须手动使用 SIMD 指令：

| 方案 | 加速比 (vs 朴素标量) | 实现 |
|------|---------------------|------|
| AVX2 `__m256` (float × 8) | ~6× | C/C++ intrinsics |
| AVX-512 (float × 16) | ~10–12× | C/C++ intrinsics |
| Rust `std::simd` / `pulp` | ~4–6× | 可移植 SIMD |

**原理**：将 8 个相邻像素的迭代打包到一个 SIMD 寄存器中并行计算。虽然不同像素逃逸时间不同，但通过 mask 逐步淘汰已逃逸的 lane。

参考：
- [SIMD tutorial @ European GNU Radio Days '24](http://web-docs.gsi.de/~mkretz/egrd24-simd/)
- [pythonspeed/mandelbrot-simd (Rust)](https://github.com/pythonspeed/mandelbrot-simd)
- [Roy Ward's optimized-mandelbrot (C/x86)](https://github.com/royward/optimized-mandelbrot)

### 1.3 软件流水线（Software Pipelining）

将多个独立像素的计算**交错执行**来隐藏 FPU 延迟：

```
// 传统：一次计算一个像素，等待结果
for each pixel: z = z² + c  (每条指令等前一条完成)

// 流水线：同时跑 4 个像素，交错指令
z1 = z1² + c1; z2 = z2² + c2; z3 = z3² + c3; z4 = z4² + c4
```

在 SIMD 基础上可再获得 ~2× 加速。此技术也称为 "sheep race optimization"。

参考：[Roy Ward's optimization article series](https://github.com/royward/optimized-mandelbrot)

### 1.4 编译器优化选项

```bash
# Go 编译器
go build -ldflags="-s -w" -gcflags="-B -d=checkptr=0" .

# C/C++
gcc -O3 -ffast-math -march=native -mtune=native -funroll-loops

# 关键：-ffast-math 放宽 IEEE 754 严格舍入，允许更多优化
```

### 1.5 各优化手段总结

| 技术 | 加速比 | 难度 | 当前项目应用 |
|------|--------|------|-------------|
| 多线程分块 | N× 核数 | 低 | ✅ 已实现（renderer.go） |
| SIMD 显式向量化 | 4–8× | 高 | ❌ Go 不支持 SIMD intrinsics |
| 软件流水线 | ~2× | 中 | ❌ |
| 编译器优化 flag | 1.5–3× | 低 | 部分（`-s -w`） |
| 周期检测提前退出 | 可变 | 中 | ❌ |

---

## 2. 双精度浮点数不够用怎么办

### 2.1 双精度的极限

- `float64` (IEEE 754 double)：53 位尾数 ≈ 15–17 位十进制有效数字
- Mandelbrot 缩放中，每 10× 放大约消耗 1 位十进制精度
- **实际极限**：约 **1e13 – 1e15** 倍放大后，double 精度耗尽，像素坐标无法区分

### 2.2 精度不足的表现

当放大到 double 极限边界时：
- 像素坐标被量化为相同的 double 值 → 图像出现"块状"
- 迭代轨迹偏离真实值 → 产生伪影
- 无法继续深入放大

参考：[How Mandelbrot images are affected by floating point precision](https://hn.svelte.dev/item/39683455)

### 2.3 解决方案层次

| 方案 | 精度（十进制位） | 性能 (vs double) | 适用缩放深度 |
|------|-----------------|-------------------|-------------|
| `float64` (double) | ~15 位 | 1× | ≤ 1e13 |
| `long double` (80-bit x87) | ~19 位 | ~1× | ≤ 1e18 |
| **Double-double** (模拟 128-bit) | ~31 位 | 3–10× 慢 | 1e13 – 1e30 |
| `__float128` (libquadmath) | ~34 位 | ~10× 慢 | 1e13 – 1e30 |
| MPFR (任意精度) | 任意 | 50–500× 慢 | ≥ 1e30 |

### 2.4 Double-Double 算术（推荐中等深度）

使用**两个 double** 模拟约 106 位尾数的浮点数（实际约 31 位十进制精度）：

```
// 伪代码：double-double 加法
dd_add(a_hi, a_lo, b_hi, b_lo):
    sum_hi = a_hi + b_hi
    sum_lo = (误差补偿项)
    return (sum_hi, sum_lo)
```

优点：比 MPFR 快 1.5–3×，精度足以覆盖大部分深度缩放。基准测试：1920×1080 深缩放渲染 2h38m（vs MPFR 参考实现的 8h40m）。

参考：[josch/mandelbrot (dd benchmark)](https://github.com/josch/mandelbrot)

---

## 3. 大数运算库是否会拖累性能，能否动态切换

### 3.1 大数库性能对比

| 库 | 语言 | 相对速度 | 特点 |
|----|------|---------|------|
| 原生 `float64` | — | 1× (基准) | 最快，精度有限 |
| Double-double (手写) | C/C++ | 3–10× 慢 | 128-bit 模拟，无外部依赖 |
| GMP `mpf_t` (C API) | C | 50–200× 慢 | 业界标准，高度优化 |
| MPFR | C | 100–500× 慢 | IEEE 兼容舍入，比 GMP 慢 4× |
| Go `math/big.Float` | Go | 200–1000× 慢 | 纯 Go，分配开销大 |

**关键发现**：
- GMP/MPFR 的 C++ 封装（`mpf_class` / `mpfr::mpreal`）在内层循环中有**致命分配开销**，比 C API 慢 2×
- Go 的 `math/big.Float` 每次运算都有堆分配，**不适合**放在每像素的内层循环中
- 大数库的函数调用接口天然**无法利用 SIMD 向量化**

参考：[FractalForums: Basic Mandelbrot render optimization](https://fractalforums.org/index.php?topic=1369.msg7177#msg7177)

### 3.2 结论：动态切换是可行且必要的

**不应该对所有像素使用大数运算**。正确的架构是 **Perturbation Theory（扰动理论）**：

```
┌─────────────────────────────────────┐
│  1. 选一个参考点 C_ref              │
│     用高精度 (MPFR/大数) 计算其轨道  │
│     → 这只需要算 1 条轨道！          │
├─────────────────────────────────────┤
│  2. 对每个像素 C = C_ref + δ        │
│     用 float64 计算 δ 的演化        │
│     → 99.999% 的计算在 fast path    │
├─────────────────────────────────────┤
│  3. 当 δ 增长过大时 (glitch)        │
│     回退到高精度重算该像素           │
│     → 仅少数像素走 slow path        │
└─────────────────────────────────────┘
```

**性能影响**：
- 大部分缩放深度：仅 **1 条参考轨道**需要高精度 → 开销可忽略
- 极端深度（>1e30）：参考轨道本身需要数千位精度 → 用多线程后台计算，不阻塞渲染

参考：[Perturbation Theory (K.I. Martin)](http://www.science.eclipse.co.uk/sft_maths.pdf)

---

## 4. 如何使用显卡加速

### 4.1 为何 GPU 是 Mandelbrot 的理想选择

- 每个像素独立 → 完美适配 GPU 数千核心并行
- 逃逸时间算法逻辑简单 → 适合 Shader
- 典型加速比：**100–1000×** vs 单核 CPU

参考基准：Numba CUDA 上 1024×1536 渲染：5.88s (CPU) → 0.01s (GPU)，约 **500× 加速**。

### 4.2 GPU 方案对比

| 方案 | 适用场景 | 双精度？ | 开发难度 | 跨平台 |
|------|---------|---------|---------|--------|
| **GLSL Compute Shader** | 实时中等缩放 | ❌ 单精度 (fp32) | 中 | ✅ OpenGL 4.3+ |
| **GLSL Fragment Shader** | 实时预览 | ❌ 单精度 | 低 | ✅ 所有平台 |
| **CUDA** | NVIDIA 深缩放 | ✅ 但很慢 (1:64 SP:DP) | 中 | ❌ NVIDIA only |
| **Vulkan Compute** | 高性能跨平台 | ❌ 单精度为主 | 高 | ✅ |
| **WebGL 2.0** | 浏览器 | ❌ 单精度 | 低 | ✅ 浏览器 |
| **Metal Compute** | macOS/iOS | ✅ | 中 | ❌ Apple only |

### 4.3 GPU 双精度困境

消费级 GPU 的双精度性能被严重阉割：
- RTX 4090：双精度仅 1.3 TFLOPS，**单精度的 1/64**
- 专业卡（Quadro/Tesla）：双精度约为单精度的 1/3

**解决方案**：
- 用**扰动理论**使 GPU 只需单精度（δ 很小，单精度足够）
- 或在 GPU 上用 **double-double** 技巧（两个 float 模拟）
- FractalShark 项目实现了自定义 `2×32 + exponent` 数值类型，在 GPU 上提供 ~48-bit 尾数

参考：
- [FractalShark - CUDA deep zoom renderer](https://github.com/mattsaccount364/FractalShark)
- [GPU-Fractals (OpenGL)](https://github.com/MathandPhysicsStuff/GPU-Fractals)

### 4.4 推荐：GLSL Fragment Shader 方案

对当前项目的简单 GPU 加速方案：

```
Vertex Shader:   绘制全屏四边形
Fragment Shader: 对每个像素执行 Mandelbrot 逃逸时间计算
                 → 输出颜色值到 framebuffer
```

优势：
- Go 可通过 Ebitengine 的 `ebiten.Shader{}` 直接嵌入 GLSL
- 或使用 `go-gl` / `glow` 绑定 OpenGL
- 零拷贝：GPU 直接渲染到屏幕，不需要 CPU→GPU 传输

---

## 5. 如何并发加速

### 5.1 当前项目实现

已实现**屏幕分块并发渲染**（[renderer.go](mandelbrot/renderer.go)）：
- 屏幕等分为 Rows × Cols 个图块
- 每块一个 goroutine 独立计算
- `sync.WaitGroup` 等待全部完成后组装

```go
for i, tile := range tiles {
    wg.Add(1)
    go func(idx int, t Tile) {
        defer wg.Done()
        results[idx] = r.renderTile(t, ...)
    }(i, tile)
}
wg.Wait()
```

### 5.2 并发策略对比

| 策略 | 粒度 | 优点 | 缺点 |
|------|------|------|------|
| **逐像素 goroutine** | 太细 | — | goroutine 开销远超计算，绝对不可用 |
| **逐行** | 中 | 简单 | 工作负载不均（行内像素耗时不同） |
| **逐块 (Tile)** ✅ | 中粗 | 均衡、缓存友好 | 当前实现 |
| **Worker Pool** | — | 控制并发数，复用 goroutine | 需要 channel 通信 |
| **GPU** | 最细 | 数千核心同时算 | 需要 GLSL/CUDA |

### 5.3 优化建议

**Work stealing**：当前静态分块可能导致某些块比其他块早完成（Mandelbrot 边界区域迭代次数高，内部区域低）。改进：

```go
// 任务队列 + work stealing：
// 1. 将所有 tile 放入 channel
// 2. N 个 worker goroutine 从 channel 取任务
// 3. 快速完成的 worker 可以取更多任务
// → 总耗时 ≈ 最慢 tile 的耗时，而非最慢分区的耗时

jobs := make(chan Tile, len(tiles))
for i := 0; i < runtime.NumCPU(); i++ {
    go worker(jobs, results)
}
for _, t := range tiles {
    jobs <- t
}
```

### 5.4 NUMA 感知

在多路 CPU 系统上，将 tile 分配到与内存最近的 NUMA 节点可减少 10–20% 延迟。Go 的 `runtime.LockOSThread()` 可绑定 goroutine 到特定核心。

---

## 6. 其他提升性能和精度的黑科技

### 6.1 Perturbation Theory（扰动理论）★ 最重要

2013 年 K.I. Martin 的突破性发明，使深缩放从不可能变为可能。

**核心思想**：
```
传统：每个像素用高精度从头算 → 极慢
扰动：选一个参考点 C_ref，高精度算其轨道
      对其他点 C = C_ref + δ，用低精度算 δ 的演化
      → 关键公式：Δ_{n+1} = 2Z_n·Δ_n + Δ_n² + δ
```

**为什么有效**：两个相近大数的差可以用低精度表示。
```
例：1,234,567 - 1,234,560 = 7  ← 低精度足够
```

参考：
- [sft_maths.pdf (原始论文)](http://www.science.eclipse.co.uk/sft_maths.pdf)
- [perturbation.pdf (Claude Heiland-Allen)](https://mathr.co.uk/mandelbrot/perturbation.pdf)

### 6.2 Series Approximation（级数近似）

在 Perturbation 基础上的进一步加速：

对前 N 次迭代，用预计算的级数 \(\Delta_n \approx A_n·δ + B_n·δ² + C_n·δ³\) 一次跳过多个迭代，不需要逐次计算。

- **Glitch 问题**：级数近似可能偏离真实轨道 → 需要 **glitch detection**（与探针像素比较）自动回退
- **Underflow 问题**：δ 极小时 δ³ 可能下溢 double → 用**缩放技巧** \(ε = Kδ\) 保持值域

参考：[Ultra Fractal forum: perturbation + bailout](https://www.ultrafractal.com/forum/index.php?u=/topic/823)

### 6.3 周期检测（Period Checking）

对 Mandelbrot 集**内部**的点（逃逸迭代 = MaxIter），轨道最终会收敛到周期环。检测到周期性即可提前终止：

```go
// 简单周期检测：
if z 回到之前某个 z_n 的邻域内 → 已进入周期 → 点在集合内部
```

配合 **Cardioid/Bulb 检测**（几何公式直接判断是否在主心形或周期-2 球内），可跳过大量内部点的迭代。

### 6.4 Mariani-Silver 矩形细分算法

利用 Mandelbrot 集的**连续性**减少计算量：

1. 将屏幕分成大矩形
2. 计算矩形**边界**上的像素
3. 如果边界像素全部相同（全在集内或全在集外）→ 矩形内部全填色，跳过
4. 如果边界像素不同 → 细分矩形，递归

可减少 50–90% 的像素计算，特别是在大面积单调区域（如主心形内部）。

参考：[Mariani-Silver Algorithm (mrob.com)](https://mrob.com/pub/muency/adjacencyoptimization.html)

### 6.5 Nano-Precision 与 NTT 加速

在极端深缩放（>1e100）时，参考轨道的精度需求达到数千位甚至更多：

- **NTT (Number Theoretic Transform)**：在 GPU 上实现大整数乘法。FractalShark 项目用 NTT 在 RTX 4090 上加速参考轨道计算，比多线程 CPU 快 **10×**，支持约 158K 位十进制精度的参考轨道。

参考：[FractalShark - GPU NTT reference orbits](https://github.com/mattsaccount364/FractalShark)

### 6.6 自适应逃逸半径控制

高 bailout 值会破坏扰动公式的稳定性。动态调整逃逸半径：

```
标准 bailout: |z| > 2.0
自适应: 根据缩放深度和 δ 的大小自动调节
```

### 6.7 各技术适用场景总结

| 技术 | 适用缩放 | 实现复杂度 | 性能收益 |
|------|---------|-----------|---------|
| CPU 多线程分块 | 全部 | ★☆☆☆☆ | N× 核数 |
| GPU Fragment Shader | ≤ 1e12 | ★★☆☆☆ | 100–500× |
| Cardioid/Bulb 快速检测 | 全部 | ★☆☆☆☆ | 视场景 10–50% |
| 周期检测 | 全部 | ★★☆☆☆ | 内部区域极大加速 |
| Mariani-Silver | 全部 | ★★★☆☆ | 50–90% |
| Perturbation Theory | > 1e13 | ★★★★☆ | 1000×+ |
| Series Approximation | > 1e15 | ★★★★★ | 10–100× |
| NTT GPU 高精度 | > 1e100 | ★★★★★ | 10× |

---

## 参考文献

1. [K.I. Martin — SuperFractalThing & Perturbation Theory (2013)](http://www.science.eclipse.co.uk/sft_maths.pdf)
2. [Claude Heiland-Allen — Perturbation Theory Perspective](https://mathr.co.uk/mandelbrot/perturbation.pdf)
3. [Robert Munafo — Encyclopedia of the Mandelbrot Set (mrob.com)](https://mrob.com/pub/muency.html)
4. [FractalShark — CUDA Deep Zoom Renderer](https://github.com/mattsaccount364/FractalShark)
5. [Roy Ward — Optimization Example: Mandelbrot Set](https://github.com/royward/optimized-mandelbrot)
6. [Zust — Mandelbrot Refinement with BigFloat](https://github.com/zhuchuanjing/zust)
7. [FractalForums — Mandelbrot optimization discussion](https://fractalforums.org/index.php?topic=1369)
8. [josch/mandelbrot — double-double benchmark](https://github.com/josch/mandelbrot)
9. [GPU-Fractals — OpenGL Real-time](https://github.com/MathandPhysicsStuff/GPU-Fractals)
10. [Mandelbrot SIMD tutorial @ GNU Radio Days '24](http://web-docs.gsi.de/~mkretz/egrd24-simd/)
