package cmd

import (
	"context"
	"fmt"
	"github.com/containers/image/v5/transports"
	"github.com/flytam/filenamify"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strings"
	"time"

	cc "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/spf13/cobra"
)

var image string
var savePath = "./"
var username string
var password string
var identityToken string
var timeout time.Duration

var insecurePolicy = []byte(`{"default":[{"type":"insecureAcceptAnything"}]}`)

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.TODO()
		image = "docker://" + image
		srcRef, err := alltransports.ParseImageName(image)
		if err != nil {
			logrus.Errorf("解析Image错误:%v", err)
			return err
		}
		dest, imageDir, err := imageDestDir(savePath, image)
		if err != nil {
			return err
		}
		//检查文件夹是否存在,不存在则创建
		if err := createDestDir(filepath.Dir(imageDir)); err != nil {
			return err
		}
		destRef, err := alltransports.ParseImageName(dest)
		if err != nil {
			return err
		}

		sourceCtx := &types.SystemContext{
			DockerAuthConfig: &types.DockerAuthConfig{
				Username:      username,
				Password:      password,
				IdentityToken: identityToken,
			},
			DockerBearerRegistryToken:   "",
			DockerRegistryUserAgent:     "",
			DockerInsecureSkipTLSVerify: types.OptionalBoolTrue,
		}
		destinationCtx := &types.SystemContext{}

		policy, err := signature.NewPolicyFromBytes(insecurePolicy)
		if err != nil {
			return err
		}
		policyContext, err := signature.NewPolicyContext(policy)
		if err != nil {
			return err
		}
		subCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		_, err = cc.Image(subCtx, policyContext, destRef, srcRef, &cc.Options{
			ReportWriter:       os.Stdout,
			SourceCtx:          sourceCtx,
			DestinationCtx:     destinationCtx,
			ImageListSelection: cc.CopySystemImage,
		})
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
	pullCmd.PersistentFlags().StringVar(&image, "image", "", "pull image name")
	pullCmd.PersistentFlags().StringVar(&savePath, "savePath", "", "pull image save path")
	pullCmd.PersistentFlags().StringVar(&username, "username", "", "pull image username")
	pullCmd.PersistentFlags().StringVar(&password, "password", "", "pull image password")
	pullCmd.PersistentFlags().StringVar(&identityToken, "identityToken", "", "pull image identityToken")
	pullCmd.PersistentFlags().DurationVar(&timeout, "timeout", time.Hour, "pull image timeout")
}

/*
path=/path
imageName=docker://imagename

docker-archive:/path/imagename.tar.gz
/path/imagename.tar.gz
*/
func imageDestDir(path string, imageName string) (string, string, error) {
	names := append(transports.ListNames(), "//")
	replaceNames := make([]string, len(names)*2)
	for i, n := range names {
		replaceNames[i*2] = n
		replaceNames[i*2+1] = ""
	}
	replacer := strings.NewReplacer(replaceNames...)
	replace := replacer.Replace(imageName)
	imageName = replace
	s, err := filenamify.Filenamify(imageName, filenamify.Options{Replacement: "-"})
	if err != nil {
		return "", "", err
	}
	filep := fmt.Sprintf("%s.tar.gz", filepath.Join(path, s))
	return fmt.Sprintf("%s:%s", archive.Transport.Name(), filep), filep, nil
}

func createDestDir(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
		return nil
	}
	return err
}
