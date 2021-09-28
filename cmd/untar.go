/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/images"
	cc "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/directory"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var imagePath string
var tempPath string
var distPath string

const manifestFileName = "manifest.json"

// untarCmd represents the untar command
var untarCmd = &cobra.Command{
	Use:   "untar",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logrus.Info("开始解压镜像...")
		ctx := context.TODO()
		//docker-archive:/path/imagename.tar.gz
		imageAbsPath, err := filepath.Abs(imagePath)
		if err != nil {
			return err
		}
		imagePath = "docker-archive:" + imageAbsPath
		policy, err := signature.NewPolicyFromBytes(insecurePolicy)
		if err != nil {
			return err
		}
		policyContext, err := signature.NewPolicyContext(policy)
		if err != nil {
			return err
		}
		//解析workdir
		if err := createDestDir(tempPath); err != nil {
			return err
		}
		destRef, err := directory.Transport.ParseReference(tempPath)
		if err != nil {
			return err
		}
		//解析
		srcRef, err := alltransports.ParseImageName(imagePath)
		if err != nil {
			return err
		}
		sourceCtx := &types.SystemContext{}
		destinationCtx := &types.SystemContext{}

		//解压镜像
		if _, err := cc.Image(ctx, policyContext, destRef, srcRef, &cc.Options{
			ReportWriter:       os.Stdout,
			SourceCtx:          sourceCtx,
			DestinationCtx:     destinationCtx,
			ImageListSelection: cc.CopySystemImage,
		}); err != nil {
			return err
		}
		fmt.Println("解压tar包完成,开始解压镜像layer")
		//解压 "层"
		content, err := ioutil.ReadFile(manifestDir(tempPath))
		if err != nil {
			return err
		}
		manifest := &v1.Manifest{}
		err = json.Unmarshal(content, manifest)
		if err != nil {
			fmt.Printf("反序列化Manifest错误,error:%v\n", err)
			return err
		}
		for _, layer := range manifest.Layers {
			switch layer.MediaType {
			case images.MediaTypeDockerSchema2LayerGzip:
				err = UnTar(getLayerFilePath(tempPath, layer.Digest), distPath)
				if err != nil {
					fmt.Printf("用tar包解压层错误,error:%v,layer:%s\n", err, layer.Digest.Encoded())
					return err
				}
			default:
				err := fmt.Errorf("unsupport image %s layer %s media type:%s", image, layer.Digest.Encoded(), layer.MediaType)
				return err
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(untarCmd)
	untarCmd.PersistentFlags().StringVar(&imagePath, "imagePath", "", "untar image name")
	untarCmd.PersistentFlags().StringVar(&tempPath, "tempPath", "", "untar image tempPath")
	untarCmd.PersistentFlags().StringVar(&distPath, "distPath", "", "untar image distPath")
}

func manifestDir(workdir string) string {
	return filepath.Join(workdir, manifestFileName)
}

func getLayerFilePath(imageDir string, digest digest.Digest) string {
	return filepath.Join(imageDir, digest.Encoded())
}

func UnTar(src string, dest string) error {
	// 打开准备解压的 tar 包
	fr, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fr.Close()

	// 将打开的文件先解压
	//gr, err := gzip.NewReader(fr)
	//if err != nil {
	//	return err
	//}
	//defer gr.Close()

	// 通过 gr 创建 tar.Reader
	tr := tar.NewReader(fr)
	// 现在已经获得了 tar.Reader 结构了，只需要循环里面的数据写入文件就可以了
	for {
		hdr, err := tr.Next()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case hdr == nil:
			continue
		}

		dstFileDir := filepath.Join(dest, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if b := ExistDir(dstFileDir); b {
				continue
			}
			if err := os.MkdirAll(dstFileDir, 0775); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := copyFile(dstFileDir, hdr, tr); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupport tar %s Typeflag %v", src, hdr.Typeflag)
		}
	}
}

func copyFile(dstFileDir string, hdr *tar.Header, tr *tar.Reader) error {
	file, err := os.OpenFile(dstFileDir, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = io.Copy(file, tr); err != nil {
		return err
	}
	return nil
}

// 判断目录是否存在
func ExistDir(dirname string) bool {
	fi, err := os.Stat(dirname)
	return (err == nil || os.IsExist(err)) && fi.IsDir()
}
