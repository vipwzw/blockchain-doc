package main

import (
	"fmt"
	"math/big"
	"os/exec"
	"strings"
	"time"
)

var (
	one = big.NewInt(1)
	two = big.NewInt(2)
	// 2^256 上限
	maxN = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
)

// 计算 p = 2^256 - 2^32 - c
func computeP(c int64) *big.Int {
	// 2^256
	p := new(big.Int).Exp(two, big.NewInt(256), nil)
	// - 2^32
	p.Sub(p, new(big.Int).Exp(two, big.NewInt(32), nil))
	// - c
	p.Sub(p, big.NewInt(c))
	return p
}

// Miller-Rabin 素数测试
func isProbablePrime(n *big.Int, k int) bool {
	return n.ProbablyPrime(k)
}

// 使用 PARI/GP 计算椭圆曲线群阶
// 曲线: y² = x³ + ax + b over F_p
func computeECOrderPARI(p *big.Int, a, b int64) (*big.Int, error) {
	// PARI/GP 脚本
	script := fmt.Sprintf(`
p = %s;
E = ellinit([0, 0, 0, %d, %d], p);
n = ellcard(E);
print(n);
quit;
`, p.String(), a, b)

	cmd := exec.Command("gp", "-q")
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("PARI/GP 执行错误: %v", err)
	}

	// 解析输出
	orderStr := strings.TrimSpace(string(output))
	if orderStr == "" {
		return nil, fmt.Errorf("PARI/GP 返回空结果")
	}

	order := new(big.Int)
	_, ok := order.SetString(orderStr, 10)
	if !ok {
		return nil, fmt.Errorf("无法解析群阶: %s", orderStr)
	}
	return order, nil
}

// 检查 PARI/GP 是否已安装
func checkPARIInstalled() bool {
	cmd := exec.Command("gp", "--version")
	err := cmd.Run()
	return err == nil
}

// 格式化大整数为十六进制（截断显示）
func formatBigIntHex(n *big.Int) string {
	hex := fmt.Sprintf("%X", n)
	if len(hex) > 32 {
		return hex[:16] + "..." + hex[len(hex)-16:]
	}
	return hex
}

// 搜索结果
type SearchResult struct {
	C     int64
	P     *big.Int
	N     *big.Int
	Found bool
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           椭圆曲线参数搜索工具 (Go + PARI/GP)                  ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  曲线方程: y² = x³ + 7                                         ║")
	fmt.Println("║  素数形式: p = 2²⁵⁶ - 2³² - c                                  ║")
	fmt.Println("║  目标: 找到使群阶 N 为素数的 c 值                              ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 检查 PARI/GP 是否安装
	if !checkPARIInstalled() {
		fmt.Println("❌ 错误: 未检测到 PARI/GP")
		fmt.Println()
		fmt.Println("请先安装 PARI/GP:")
		fmt.Println("  macOS:  brew install pari")
		fmt.Println("  Ubuntu: sudo apt install pari-gp")
		fmt.Println("  Arch:   sudo pacman -S pari")
		return
	}
	fmt.Println("✓ PARI/GP 已安装")
	fmt.Println()

	// 曲线参数 a=0, b=7 (和 secp256k1 相同)
	var a, b int64 = 0, 7

	// 搜索参数
	startC := int64(998)
	endC := int64(1000000)

	fmt.Printf("搜索范围: c ∈ [%d, %d]\n", startC, endC)
	fmt.Println()
	fmt.Println("开始搜索...")
	fmt.Println(strings.Repeat("-", 70))

	startTime := time.Now()
	primeCount := 0
	testedCount := 0

	for c := startC; c <= endC; c++ {
		p := computeP(c)

		// 1. 检查 p 是否是素数
		if !isProbablePrime(p, 20) {
			continue
		}

		primeCount++
		testedCount++
		fmt.Printf("\n[%d] c = %d\n", testedCount, c)
		fmt.Printf("    p = %s\n", formatBigIntHex(p))
		fmt.Printf("    p 是素数 ✓\n")

		// 2. 计算椭圆曲线群阶 N
		fmt.Printf("    计算群阶 N (Schoof 算法)...")
		calcStart := time.Now()

		N, err := computeECOrderPARI(p, a, b)
		if err != nil {
			fmt.Printf(" 失败\n")
			fmt.Printf("    错误: %v\n", err)
			continue
		}

		calcTime := time.Since(calcStart)
		fmt.Printf(" 完成 (%.2fs)\n", calcTime.Seconds())
		fmt.Printf("    N = %s\n", formatBigIntHex(N))

		// 3. 检查 N 是否小于 2^256
		if N.Cmp(maxN) >= 0 {
			fmt.Printf("    N >= 2^256，跳过 ✗\n")
			continue
		}
		fmt.Printf("    N < 2^256 ✓\n")

		// 4. 检查 N 是否是素数
		if isProbablePrime(N, 20) {
			fmt.Println()
			fmt.Println(strings.Repeat("=", 70))
			fmt.Println("                        🎉 找到了！🎉")
			fmt.Println(strings.Repeat("=", 70))
			fmt.Println()
			fmt.Printf("c = %d\n", c)
			fmt.Println()
			fmt.Println("p (十进制):")
			fmt.Println(p.String())
			fmt.Println()
			fmt.Println("p (十六进制):")
			fmt.Printf("0x%X\n", p)
			fmt.Println()
			fmt.Println("N (十进制):")
			fmt.Println(N.String())
			fmt.Println()
			fmt.Println("N (十六进制):")
			fmt.Printf("0x%X\n", N)
			fmt.Println()
			fmt.Println("曲线方程: y² = x³ + 7")
			fmt.Println()

			// 计算 p - N 的差值
			diff := new(big.Int).Sub(p, N)
			fmt.Printf("p - N = %s\n", diff.String())
			fmt.Println()

			// Hasse 边界验证
			// |N - (p+1)| <= 2*sqrt(p)
			pPlus1 := new(big.Int).Add(p, one)
			hasseDiff := new(big.Int).Sub(N, pPlus1)
			hasseDiff.Abs(hasseDiff)
			fmt.Printf("|N - (p+1)| = %s\n", hasseDiff.String())
			fmt.Println("(应 <= 2√p ≈ 2^129)")
			fmt.Println()

			totalTime := time.Since(startTime)
			fmt.Printf("搜索用时: %.2f 秒\n", totalTime.Seconds())
			fmt.Printf("测试了 %d 个素数 p\n", testedCount)
			fmt.Println(strings.Repeat("=", 70))
			return
		}

		fmt.Printf("    N 不是素数 ✗\n")

		// 显示 N 的一些因子信息
		if N.Bit(0) == 0 {
			fmt.Printf("    N 是偶数 (被 2 整除)\n")
		}
	}

	totalTime := time.Since(startTime)
	fmt.Println()
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("搜索完成，未在 c ∈ [%d, %d] 范围内找到合适的参数\n", startC, endC)
	fmt.Printf("共检测了 %d 个素数 p\n", primeCount)
	fmt.Printf("总用时: %.2f 秒\n", totalTime.Seconds())
}
