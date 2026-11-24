# puzzle

基于 [ebiten](https://github.com/hajimehoshi/ebiten) 开发的 3x3 数字华容道（8-puzzle）小游戏。

## 玩法

- 游戏区为 3x3 棋盘（300x300 像素），开局时数字 1~8 随机分布在棋盘上，
  空格固定在右下角（洗牌保证每局必有解）。
- 鼠标左键点击与空格**上下左右相邻**的方块，方块会平滑滑入空格；
  不相邻则无反应。
- 将数字 1~8 按「从左到右、从上到下」的顺序排好、空格回到右下角即过关，
  过关后右下角自动补上数字 **9**，棋盘锁定。
- 底部状态栏实时显示本局**步数**与**耗时（秒）**，过关后冻结；
  点击 **[再来一局]** 重新洗牌开始新局。
- 窗口大小固定为 300x340（300x300 游戏区 + 40px 状态栏），不可拉伸。

## 构建与运行

```bash
make            # 构建 Linux (WSL) 版本 → dist/puzzle-linux-amd64
make windows    # 构建 Windows 版本     → dist/puzzle-windows-amd64.exe
make all        # 两个平台一起构建
make test       # 运行单元测试
make run        # 编译并直接运行
make clean      # 清理构建产物
```

编译说明：

- **Linux**：本仓库所在的 WSL2 环境缺少 GL/X11 的 C 开发头文件，因此
  使用 ebiten 的 `purego` 纯 Go 编译模式（运行时 dlopen WSLg 自带的系统库）。
  若已安装编译依赖（Ubuntu/Debian）：
  `sudo apt install libgl1-mesa-dev xorg-dev libasound2-dev`，
  可在 Makefile 中去掉 purego 标签改用普通编译。
- **Windows**：从 Linux 直接交叉编译，产物为 PE32+ GUI 程序，拷贝到
  Win11 即可双击运行（`-H=windowsgui` 保证不弹控制台窗口，调试期可在
  Makefile 中去掉该选项以查看日志）。
- **中文字体**：运行时从系统加载——Linux 用 Noto Sans CJK（文泉驿兜底），
  Windows 用微软雅黑（黑体兜底），无需在仓库中内置字体文件。

## 测试

```bash
make test
```

覆盖洗牌有解性、相邻判定、计步、过关判定、计时冻结、重开重置等逻辑。

## 开发辅助

设置环境变量可在启动约 1.5 秒后把游戏画面截图保存为 PNG 并自动退出，
用于自动化视觉验证：

```bash
PUZZLE_DUMP=/tmp/shot.png make run
```

## 项目结构

```
main.go       游戏全部实现（棋盘、洗牌、动画、状态栏、字体加载）
main_test.go  逻辑单元测试
Makefile      构建脚本（Linux / Windows 双平台）
dist/         构建产物输出目录
```
