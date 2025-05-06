package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/1Panel-dev/1Panel/backend/app/dto"
	"github.com/pkg/errors"
)

const (
	Mode                 = "dev"
	AppRepo              = "https://apps.1panel.pro"
	DockerRegistry       = "docker-registry:5000"
	OnlineDockerRegistry = "docker.1panel.live"
)

var (
	downloadType    = flag.String("type", "images", "<composeFile|images>")
	localAssertsDir = flag.String("dir", "/usr/local/deploy/magic-runner/asserts/cache", "your local asserts dir")
	jsonPath        = flag.String("json", "/opt/1panel/resource/1panel.json", "/path/to/1panel.json")
)

func main() {
	flag.Parse()

	if downloadType == nil || *downloadType == "" ||
		jsonPath == nil || *jsonPath == "" ||
		localAssertsDir == nil || *localAssertsDir == "" {
		flag.PrintDefaults()
		return
	}

	os.MkdirAll(*localAssertsDir, os.FileMode(0666))

	if _, err := os.Stat(*jsonPath); err != nil {
		panic(fmt.Sprintf("%+v", err))
	}

	buf, err := os.ReadFile(*jsonPath)
	if err != nil {
		panic(fmt.Sprintf("%+v", err))
	}

	var appJson map[string]any
	var appList dto.AppList
	if err := json.Unmarshal(buf, &appJson); err != nil {
		panic(fmt.Sprintf("%+v", err))
	}

	if err := json.Unmarshal(buf, &appList); err != nil {
		panic(fmt.Sprintf("%+v", err))
	}

	switch *downloadType {
	case "composeFile":
		DownloadComposeFile(appJson)
	case "images":
		DownloadImages(&appList)
	}
}

func DownloadImages(appList *dto.AppList) {
	composeTarPaths := make([]string, 0)
	composeFileContents := make([]string, 0)
	images := make([]string, 0)

	for _, app := range appList.Apps {
		appKey := app.AppProperty.Key

		if appKey == "" {
			panic(errors.Errorf("appKey can't be empty, %+v", app))
		}

		for _, version := range app.Versions {
			if version.Name == "" {
				panic(errors.Errorf("versoin.Name can't be empty, %+v", version))
			}

			composeTarPath, err := BuildAssertPath(version.DownloadUrl)
			if err != nil {
				fmt.Printf("BuildAssertPath error, %+v", err)
				continue
			}

			composeTarPaths = append(composeTarPaths, composeTarPath)
			// images = append(images, fmt.Sprintf("%s:%s", appKey, version.Name))
		}
	}

	wg := &sync.WaitGroup{}

	// 限制并行下载的数量
	downloadQueue := make(chan any, 1)

	// 下载镜像
downloadImagesLabel:
	for _, composeTarPath := range composeTarPaths {
		tarFile, err := os.OpenFile(composeTarPath, os.O_RDONLY, os.FileMode(0666))
		if err != nil {
			fmt.Printf("%+v\n", errors.WithStack(err))
			continue
		}
		defer tarFile.Close()

		gr, err := gzip.NewReader(tarFile)
		if err != nil {
			fmt.Printf("%+v\n", errors.WithStack(err))
			continue
		}
		defer gr.Close()

		var composeFileContent string

		// 查找docker-compose.yml文件
		tr := tar.NewReader(gr)
		for {
			header, err := tr.Next()
			if err != nil {
				if err == io.EOF {
					break
				} else {
					fmt.Printf("%+v\n", errors.WithStack(err))
					continue downloadImagesLabel
				}
			}

			if !strings.Contains(header.Name, "docker-compose.yml") {
				continue
			}

			composeFileBuf, err := io.ReadAll(tr)
			if err != nil {
				fmt.Printf("%+v\n", errors.WithStack(err))
				continue downloadImagesLabel
			}

			composeFileContent = string(composeFileBuf)
			break
		}

		composeFileContents = append(composeFileContents, composeFileContent)
		expr := regexp.MustCompile("image: (.*)")
		matchs := expr.FindStringSubmatch(composeFileContent)
		if len(matchs) < 2 {
			fmt.Printf("%+v\n", errors.Errorf("image label not exists!\n%+v\n", composeFileContent))
			continue
		}

		// fullImageName := fmt.Sprintf("%s/%s", OnlineDockerRegistry, matchs[1])
		fullImageName := matchs[1]
		images = append(images, fullImageName)

		wg.Add(1)
		go func() {
			/// 临时跳过无法下载的镜像
			// if strings.Contains(fullImageName, "toeverything/affine-graphql") {
			// 	return
			// }
			select {
			case downloadQueue <- nil:
			case <-time.After(time.Minute * 1):
				fmt.Printf("wait download token timeout!\n")
			}
			defer func() {
				fmt.Printf("End download image %s\n", fullImageName)
				<-downloadQueue
				wg.Done()
			}()

			fmt.Printf("Start download image %s\n", fullImageName)
			if out, err := ExecWithTimeOut("docker pull "+fullImageName, 1*time.Minute); err != nil {
				if out != "" {
					fmt.Printf("%+v\n", errors.Wrap(err, out))
					return
				}

				fmt.Printf("%+v\n", errors.WithStack(err))
			}
		}()
	}

	wg.Wait()

	// 导出镜像
	for _, image := range images {
		wg.Add(1)

		fullImageName := fmt.Sprintf("%s/%s", DockerRegistry, image)

		go func() {
			savePath := filepath.Join(*localAssertsDir, "images", strings.Replace(image, ":", "-", 1))

			fmt.Printf("Start save image %s to %s\n", fullImageName, savePath)
			if out, err := ExecWithTimeOut(fmt.Sprintf("docker save -o %s %s", savePath, fullImageName), 60*time.Minute); err != nil {
				if out != "" {
					panic(errors.Wrap(err, out))
				}

				panic(err)
			}

			wg.Done()
		}()
	}

	wg.Wait()

	fmt.Printf("Download images successful!\n")
}

func DownloadComposeFile(appJson map[string]any) {
	apps, ok := appJson["apps"].(([]any))
	if !ok {
		panic("apps is not valid")
	}

	downloadUrls := make([]string, 0, len(apps))

	for _, app := range apps {
		app := app.(map[string]any)
		versions, ok := app["versions"].([]any)
		if !ok {
			panic("versions is not valid")
		}

		for _, version := range versions {
			version := version.(map[string]any)
			downloadUrl, ok := version["downloadUrl"].(string)
			if !ok {
				panic("downloadUrl is not valid")
			}

			if downloadUrl == "" {
				continue
			}

			downloadUrls = append(downloadUrls, downloadUrl)
		}
	}

	wg := &sync.WaitGroup{}

	for _, downloadUrl := range downloadUrls {
		wg.Add(1)
		go func() {
			if err := DownloadAndSaveFile(downloadUrl); err != nil {
				fmt.Printf("download composeFile %+v, err: %+v\n", downloadUrl, err)
			}
			wg.Done()
		}()
	}

	wg.Wait()
}

func DownloadAndSaveFile(downloadUrl string) error {
	prefix := Mode + "/1panel"
	idx := strings.Index(downloadUrl, prefix)
	downloadUrl = AppRepo + "/dev/1panel" + downloadUrl[idx+len(prefix):]
	resp, err := http.Get(downloadUrl)
	if err != nil {
		return errors.WithStack(err)
	}

	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := SaveLocalAssert(downloadUrl, buf); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func SaveLocalAssert(url string, buf []byte) error {
	fmt.Printf("Start save local assert, url: %v\n", url)
	filePath, err := BuildAssertPath(url)
	if err != nil {
		return errors.WithStack(err)
	}

	logoDir := filepath.Dir(filePath)
	if err := os.MkdirAll(logoDir, os.FileMode(0666)); err != nil {
		return errors.WithStack(err)
	}

	logoFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, os.FileMode(0666))
	if err != nil {
		return errors.WithStack(err)
	}

	defer logoFile.Close()

	if _, err = logoFile.Write(buf); err != nil {
		return errors.WithStack(err)
	}

	fmt.Printf("Save local assert successful, url: %v\n", url)
	return nil
}

func BuildAssertPath(url string) (string, error) {
	assertPrefix := Mode + "/" + "1panel"
	assertPrefixIdx := strings.Index(url, assertPrefix)
	assertEndIdx := assertPrefixIdx + len(assertPrefix)

	if assertEndIdx >= len(url) {
		return "", errors.New("Error Icon Path")
	}

	logoPath := url[assertEndIdx:]
	return filepath.Join(*localAssertsDir, logoPath), nil
}

func ExecWithTimeOut(cmdStr string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// var stdout, stderr bytes.Buffer
	// cmd.Stdout = &stdout
	// cmd.Stderr = &stderr
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", errors.New("Context timeout")
	}
	if err != nil {
		return "", errors.WithStack(err)
	}
	// if err != nil {
	// 	return handleErr(stdout, stderr, errors.WithStack(err))
	// }
	// return stdout.String(), nil
	return "", nil
}

func handleErr(stdout, stderr bytes.Buffer, err error) (string, error) {
	errMsg := ""
	if len(stderr.String()) != 0 {
		errMsg = fmt.Sprintf("stderr: %s", stderr.String())
	}
	if len(stdout.String()) != 0 {
		if len(errMsg) != 0 {
			errMsg = fmt.Sprintf("%s; stdout: %s", errMsg, stdout.String())
		} else {
			errMsg = fmt.Sprintf("stdout: %s", stdout.String())
		}
	}
	return errMsg, err
}
