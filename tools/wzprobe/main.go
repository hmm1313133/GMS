// wzprobe 是 P3.1 的一次性探测工具：打印 wz 文件头与目录树概览。
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)

func main() {
	path := flag.String("f", "", "wz 文件路径")
	n := flag.Int("n", 96, "打印前 N 字节")
	flag.Parse()
	if *path == "" {
		fmt.Fprintln(os.Stderr, "用法: wzprobe -f <file.wz> [-n 96]")
		os.Exit(2)
	}
	st, err := os.Stat(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stat:", err)
		os.Exit(1)
	}
	fmt.Printf("file=%s size=%d\n", *path, st.Size())
	f, err := os.Open(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer f.Close()
	buf := make([]byte, *n)
	got, err := f.Read(buf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
	buf = buf[:got]
	fmt.Println("hex:")
	fmt.Println(hex.Dump(buf))
	fmt.Printf("ident=%q\n", string(buf[0:4]))
}
