# 椭圆曲线参数搜索工具

基于 secp256k1 曲线形式，搜索新的椭圆曲线参数。

## 目标

- 曲线方程: `y² = x³ + 7`（与 secp256k1 相同）
- 素数形式: `p = 2²⁵⁶ - 2³² - c`
- 目标: 找到使群阶 `N` 为素数的 `c` 值

## 依赖

需要安装 PARI/GP（用于计算椭圆曲线群阶的 Schoof 算法）：

```bash
# macOS
brew install pari

# Ubuntu/Debian
sudo apt install pari-gp

# Arch Linux
sudo pacman -S pari

# 验证安装
gp --version
```

## 运行

```bash
cd ec_search
go run main.go
```

## 原理

1. 从 `c = 998` 开始遍历
2. 计算 `p = 2²⁵⁶ - 2³² - c`
3. 检查 `p` 是否为素数（Miller-Rabin）
4. 如果 `p` 是素数，使用 PARI/GP 的 `ellcard()` 函数计算椭圆曲线群阶 `N`
5. 检查 `N` 是否为素数
6. 如果 `N` 是素数，则找到目标曲线参数

## 为什么需要 N 是素数？

- 素数阶的群一定是循环群
- 循环群适合密码学应用（离散对数问题）
- 没有小子群攻击的风险

## secp256k1 参考

```
secp256k1 参数:
c = 977
p = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
N = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
```
