package thrift

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"git.xiaojukeji.com/gulfstream/thriftpp"
	"github.com/spf13/cobra"
)

// VERSION 发版记得更新版本号!
const VERSION = "v0.0.1"

var (
	dirs               []string
	idls               []string
	outputDir          string
	language           string
	singleMode         bool
	syntaxCheckOnly    bool
	proto3WithOptional bool
	keyWordsFile       string
)

func main() {
	rootCmd := cobra.Command{
		Use:   "thriftpp",
		Short: "convert thrift IDL to protobuf3",
		Run: func(cmd *cobra.Command, args []string) {
			for _, idl := range idls {
				codes, err := convertIDL(dirs, idl, language, keyWordsFile, singleMode, syntaxCheckOnly, proto3WithOptional)
				if err != nil {
					fmt.Printf("[Error]: %s\n", err)
					os.Exit(1)
				}
				if syntaxCheckOnly {
					return
				}

				err = os.MkdirAll(outputDir, os.ModePerm)
				if err != nil {
					fmt.Printf("[Error]: %s\n", err)
					os.Exit(1)
				}
				if len(codes) == 0 {
					fmt.Println("[WARN] nothing generated")
					os.Exit(1)
				}
				for k, v := range codes {
					f := filepath.Join(outputDir, k) + ".proto"
					fd, err := os.OpenFile(f, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0666)
					if err != nil {
						fmt.Printf("[Error]: %s\n", err)
						os.Exit(1)
					}
					_, err = fd.WriteString(v)
					if err != nil {
						fmt.Printf("fail to write file %s, due to %s", f, err)
						os.Exit(1)
					}
					_ = fd.Close()
				}
			}
			fmt.Println("Done!")
		},
	}
	rootCmd.Flags().BoolVarP(&singleMode, "single", "s", false, "单文件模式，不递归解析include")
	rootCmd.PersistentFlags().BoolVarP(&syntaxCheckOnly, "check", "c", false, "syntax check only")
	rootCmd.Flags().StringVarP(&language, "lang", "l", "go", "指定thrift IDL里namespace的语言")
	rootCmd.PersistentFlags().StringArrayVarP(&idls, "idls", "f", []string{""}, "specify thrift file to convert")
	rootCmd.MarkFlagRequired("idls")
	rootCmd.PersistentFlags().StringArrayVarP(&dirs, "dir", "I", []string{"."}, "specify dirs to search included idl file")
	rootCmd.PersistentFlags().StringVarP(&outputDir, "output", "o", "gen-pb", "specify directory to store generated pb file(s)")
	rootCmd.Flags().BoolVarP(&proto3WithOptional, "proto3-with-optional", "p", false, "转换到pb时携带optional属性")
	rootCmd.PersistentFlags().StringVarP(&keyWordsFile, "keyWords", "k", "", `Reserved words in json type, json key should be "keyWords"`)

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(CheckCmd)
	rootCmd.Execute()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of thriftpp",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(VERSION)
	},
}

var CheckCmd = &cobra.Command{
	Use:   "checkKeys",
	Short: "Chcek KeyWords in idl",
	Run: func(cmd *cobra.Command, args []string) {
		for _, idl := range idls {
			_, err := convertIDL(dirs, idl, language, keyWordsFile, singleMode, syntaxCheckOnly, proto3WithOptional)
			if err != nil {
				fmt.Println(err)
			}
		}
	},
}

func convertIDL(dirs []string, entry, lang string, keywordsFile string, singleMode, syntaxCheckOnly, proto3WithOptional bool) (map[string]string, error) {
	fd, err := os.Open(entry)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	data, err := ioutil.ReadAll(fd)
	if err != nil {
		return nil, err
	}
	result, err := thriftpp.Analysis(string(data), dirs, singleMode)
	if err != nil {
		return nil, err
	}

	if err := result.CheckKeyWord(keywordsFile); err != nil {
		return nil, err
	}

	if syntaxCheckOnly {
		return nil, nil
	}

	idx := strings.LastIndexByte(entry, '.')
	if idx != -1 {
		entry = entry[:idx]
	}

	idx = strings.LastIndexAny(entry, "/")
	if idx != -1 {
		entry = entry[idx:]
	}
	return result.CodeGen(entry, lang, true, thriftpp.CodeGenOpt{IsProto3WithOptional: proto3WithOptional})
}
