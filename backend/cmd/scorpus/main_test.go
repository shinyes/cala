package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCorpusIsUpToDate 是 CI 语料门禁的**生成端护栏**（规格 §5.5.5）。
//
// 它重新生成一次语料并与已提交的文件逐字节比对。这样两类问题都会在 CI 暴露：
//
//  1. 有人改了 Go 侧 scoring 却忘了重新生成语料
//     —— 否则 Dart 仍对着**旧**语料验证，跨端分歧会被静默放过；
//  2. 有人手工编辑了生成物
//     —— 生成物一旦可被手工修改，它就不再是 Go 实现的忠实投影。
func TestCorpusIsUpToDate(t *testing.T) {
	const relOut = "../../../app/test/scoring/corpus_generated.dart"

	committed, err := os.ReadFile(relOut)
	if err != nil {
		t.Fatalf("读取已提交语料失败（路径是否变更？）: %v", err)
	}

	tmp := filepath.Join(t.TempDir(), "corpus_generated.dart")
	cmd := exec.Command("go", "run", "./", "-out", tmp, "-min", "10000")
	cmd.Dir = "."
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("生成语料失败: %v\n%s", err, stderr.String())
	}

	fresh, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("读取新生成语料失败: %v", err)
	}

	if !bytes.Equal(committed, fresh) {
		t.Errorf("语料与 Go 侧实现不一致：已提交 %d 字节，重新生成 %d 字节。\n"+
			"请运行：cd backend && go run ./cmd/scorpus -out ../app/test/scoring/corpus_generated.dart\n"+
			"（若刚改动了 internal/scoring，必须重新生成语料，否则 Dart 侧仍在校验旧行为）",
			len(committed), len(fresh))
	}
}

// TestCorpusIsPureASCII 保证生成物不受源文件编码影响。
//
// 语料包含不可见字符（NBSP、全角空格、各类 Unicode 减号）与中文答案；
// 若原样写出，文件一旦被以其他编码打开并保存就会静默损坏。
// 本项目已因编码问题踩过坑（计划 P-R9），生成物更应绝缘于此。
func TestCorpusIsPureASCII(t *testing.T) {
	const relOut = "../../../app/test/scoring/corpus_generated.dart"

	data, err := os.ReadFile(relOut)
	if err != nil {
		t.Fatalf("读取语料失败: %v", err)
	}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		t.Error("生成物不应带 UTF-8 BOM")
	}
	for i, b := range data {
		if b > 0x7E {
			line := bytes.Count(data[:i], []byte("\n")) + 1
			t.Fatalf("第 %d 行出现非 ASCII 字节 0x%02X；所有非 ASCII 字符都应转义为 \\u{XXXX}", line, b)
		}
	}
}

// TestGeneratedCorpusSize 记录语料规模，防止有人无意中把语料砍小。
func TestGeneratedCorpusSize(t *testing.T) {
	const relOut = "../../../app/test/scoring/corpus_generated.dart"

	data, err := os.ReadFile(relOut)
	if err != nil {
		t.Fatalf("读取语料失败: %v", err)
	}
	classify := bytes.Count(data, []byte("ClassifyVector(input:"))
	compare := bytes.Count(data, []byte("CompareVector(input:"))
	total := classify + compare

	t.Logf("语料规模: 分类 %d 条, 比较 %d 条, 合计 %d 条", classify, compare, total)

	if total < 10000 {
		t.Errorf("语料合计 %d 条，低于门禁下限 10000 条", total)
	}
	// 分类语料必须覆盖「期望报错」的一侧，否则测不出静默降级
	if classify < 100 {
		t.Errorf("分类语料仅 %d 条，覆盖不足", classify)
	}
}
