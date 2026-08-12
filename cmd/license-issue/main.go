// cmd/license-issue/main.go
// 插件许可证签发工具（M3.5，作者侧）：生成 Ed25519 密钥对 + 签发 license.jwt。
//
// 用法：
//   # 1. 生成密钥对（私钥作者自持，公钥随 .bpk 包分发——安装时登记）
//   go run ./cmd/license-issue keygen -out ed25519_private.pem -pubout ed25519_public.pem
//
//   # 2. 签发许可证（付费插件购买后给站点发放）
//   go run ./cmd/license-issue sign \
//     -sub plugin:demo-plugin -licensee 站点ID -edition pro -features demo_pro \
//     -exp 1752537600 -key ed25519_private.pem -out license.jwt
//
// license.jwt 格式（对齐 docs/architecture.md 6.5.6）：
//   {sub, licensee, edition, features, exp, signature: base64(ed25519)}
//   签名消息 = 去掉 signature 的规范化 JSON（签发/验签同一结构序列化）。
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/plugin/license"
)

// keygen 生成 Ed25519 密钥对（私钥 PKCS8 0600 / 公钥 PKIX）。
func keygen(privPath string, pubPath string) error {
	priv, pub, err := license.GenerateKeyPair()
	if err != nil {
		return err
	}
	if err := license.SavePrivateKey(privPath, priv); err != nil {
		return err
	}
	if err := license.SavePublicKey(pubPath, pub); err != nil {
		return err
	}
	fmt.Printf("[成功] 密钥对已生成：\n  私钥 %s（作者自持，请妥善保管）\n  公钥 %s（随 .bpk 包分发）\n", privPath, pubPath)
	return nil
}

// sign 签发许可证（读取私钥 → 签名 → 输出 license.jwt）。
func sign(sub string, licensee string, edition string, features string, exp int64, keyPath string, outPath string) error {
	priv, err := license.LoadPrivateKey(keyPath)
	if err != nil {
		return err
	}
	var featureList []string
	if features != "" {
		featureList = strings.Split(features, ",")
	}
	raw, err := license.Sign(priv, &license.License{
		Sub: sub, Licensee: licensee, Edition: edition,
		Features: featureList, ExpiresAt: exp,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, raw, 0o644); err != nil {
		return fmt.Errorf("写入许可证失败：%w", err)
	}
	expText := "永久"
	if exp > 0 {
		expText = time.Unix(exp, 0).Format("2006-01-02 15:04")
	}
	fmt.Printf("[成功] 许可证已签发：%s（%s · 有效期至 %s）\n", outPath, edition, expText)
	return nil
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("用法：license-issue keygen -out <私钥> -pubout <公钥> | sign -sub <plugin:xxx> -licensee <站点ID> -edition <pro> [-features a,b] [-exp <ts>] -key <私钥> -out <license.jwt>")
		os.Exit(1)
	}
	subCmd := args[0]
	args = args[1:]

	switch subCmd {
	case "keygen":
		fs := flag.NewFlagSet("keygen", flag.ExitOnError)
		privPath := fs.String("out", "", "私钥输出路径（PKCS8 PEM）")
		pubPath := fs.String("pubout", "", "公钥输出路径（PKIX PEM）")
		if err := fs.Parse(args); err != nil {
			os.Exit(1)
		}
		if *privPath == "" || *pubPath == "" {
			fmt.Println("[失败] keygen 需要 -out（私钥）与 -pubout（公钥）")
			os.Exit(1)
		}
		if err := keygen(*privPath, *pubPath); err != nil {
			fmt.Printf("[失败] %v\n", err)
			os.Exit(1)
		}
	case "sign":
		fs := flag.NewFlagSet("sign", flag.ExitOnError)
		sub := fs.String("sub", "", "许可证主体（如 plugin:seo-helper）")
		licensee := fs.String("licensee", "", "被许可方（站点 ID）")
		edition := fs.String("edition", "pro", "版本（free/pro）")
		features := fs.String("features", "", "授权功能（逗号分隔）")
		expStr := fs.String("exp", "", "到期时间戳（Unix 秒；空=永久）")
		keyPath := fs.String("key", "", "私钥路径（作者自持）")
		outPath := fs.String("out", "license.jwt", "输出路径")
		if err := fs.Parse(args); err != nil {
			os.Exit(1)
		}
		if *sub == "" || *keyPath == "" {
			fmt.Println("[失败] sign 需要 -sub 与 -key")
			os.Exit(1)
		}
		var exp int64
		if *expStr != "" {
			v, err := strconv.ParseInt(*expStr, 10, 64)
			if err != nil {
				fmt.Println("[失败] -exp 应为 Unix 时间戳（数字）")
				os.Exit(1)
			}
			exp = v
		}
		if err := sign(*sub, *licensee, *edition, *features, exp, *keyPath, *outPath); err != nil {
			fmt.Printf("[失败] %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Println("[失败] 未知子命令：" + subCmd)
		os.Exit(1)
	}
}
