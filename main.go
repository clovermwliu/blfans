package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/anaskhan96/soup"
	"golang.org/x/sync/semaphore"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var SID string

type Model struct {
	Name string
	Url  string
	Cls  string
	No   string
	//相册列表
	Albums []*Album
	//视频列表
	Videos []*Video
}

type Album struct {
	No     int
	Url    string
	Photos []*Photo
}
type Photo struct {
	Name string
	Url  string
	Dir  string //存储的目录
}

type Video struct {
	No   int
	Url  string
	Name string
	Dir  string
}

var W *semaphore.Weighted //信号量

func main() {
	//2. 获取flags
	dir := flag.String("dir", ".", "blfans --dir=[store path]")
	t := flag.String("type", "all", "blfans --type=[all|latest]")
	threads := flag.Int("thread", 1, "blfans --thread=[thread count]")
	flag.Parse()

	//先登录一下
	_, _ = httpGet("http://www.beautyleg.com/member/index.php", time.Second*30)

	W = semaphore.NewWeighted(int64(*threads))
	//var model []*Model
	if *t == "all" {
		getAllModels(*dir)
	} else if *t == "latest" {
	}
	//6. 下载文件

}

func getAllModels(dir string) []*Model {
	//2. 获取model列表
	model, err := getModelList("http://www.beautyleg.com/model_list.php")
	if err != nil {
		fmt.Printf("failed to get model list, error: %v", err.Error())
		return nil
	}

	//3. 获取每一个model detail
	for i, m := range model {
		album, video, err := getModelDetail(m.Url)
		if err != nil {
			fmt.Printf("faield to get model detail, name: %v, error: %v", m.Name, err.Error())
			continue
		}

		//4. 获取每一个图片和视频的下载链接
		for ai, a := range album {
			photo, err := getAlbumDetail(a.Url)
			if err != nil {
				fmt.Printf("faield to get photo detail, name: %v, error: %v", m.Name, err.Error())
				continue
			}
			album[ai].Photos = photo
		}
		for vi, v := range video {
			name, url, err := getVideoDetail(v.Url)
			if err != nil {
				fmt.Printf("faield to get video detail, name: %v, error: %v", m.Name, err.Error())
				continue
			}
			video[vi].Name = name
			video[vi].Url = url
		}

		//5. 保存起来
		model[i].Albums = album
		model[i].Videos = video

		for _, a := range m.Albums {
			storeDir := fmt.Sprintf("%v/%v-%v", dir, a.No, m.Name)
			for _, p := range a.Photos {
				_ = downloadFile(storeDir, p.Name, p.Url, time.Hour)
			}
		}

		for _, v := range m.Videos {
			storeDir := fmt.Sprintf("%v/video", dir)
			_ = downloadFile(storeDir, v.Name, v.Url, time.Hour)
		}
	}
	return model
}

func getModelList(url string) ([]*Model, error) {
	//1. 发起请求
	res, err := httpGet(url, time.Second*time.Duration(30))
	//res, err := readTestFile("/Users/didi/Desktop/Desktop/BEAUTYLEG 模特兒列表.html")
	if err != nil {
		return nil, err
	}

	//2. 解析html
	doc := soup.HTMLParse(string(res))
	models := make([]*Model, 0)
	//3. 找到所有的tr
	trs := doc.Find("table").Find("tbody").FindAll("tr")
	for i, tr := range trs {
		if i == 0 {
			continue
		}
		tds := tr.FindAll("td")
		for _, td := range tds {
			url := td.Find("a").Attrs()["href"]
			name := td.Find("br").FindNextSibling().HTML()
			models = append(models, &Model{Name: name, Url: fmt.Sprintf("%v%v", "http://www.beautyleg.com", url)})
		}
	}
	return models, nil
}

func getModelDetail(url string) ([]*Album, []*Video, error) {
	//1. 发起请求
	res, err := httpGet(url, time.Second*time.Duration(30))
	//res, err := readTestFile("/Users/didi/Desktop/Desktop/model.html")
	if err != nil {
		return nil, nil, err
	}

	//2. 解析响应
	doc := soup.HTMLParse(string(res))
	//3. 找到所有的table
	tbs := doc.FindAll("table")
	var atb, vtb soup.Root
	for _, tb := range tbs {
		//检查第一行的标题
		t := tb.Find("tr").Find("td").Text()
		if strings.Contains(t, "Album") {
			//相册
			atb = tb
			continue
		}

		if strings.Contains(t, "Movies") {
			//视频
			vtb = tb
			continue
		}
	}

	//4. 解析相册
	var albums = make([]*Album, 0)
	var videos = make([]*Video, 0)
	if atb.Pointer != nil {
		trs := atb.FindAll("tr")
		for i, tr := range trs {
			if i == 0 { //第一行忽略
				continue
			}

			as := tr.Find("td").FindAll("a")
			for _, a := range as {
				url := a.Attrs()["href"]
				id, _ := strconv.Atoi(getFieldValueFromUrl(url, "no"))
				if id == 0 {
					continue
				}
				//保存起来
				albums = append(albums, &Album{No: id, Url: fmt.Sprintf("%v%v", "http://www.beautyleg.com", url)})
			}
		}
	}

	//5. 解析视频
	if vtb.Pointer != nil {
		trs := vtb.FindAll("tr")
		for i, tr := range trs {
			if i == 0 { //第一行忽略
				continue
			}

			as := tr.Find("td").FindAll("a")
			for _, a := range as {
				url := a.Attrs()["href"]
				id, _ := strconv.Atoi(getFieldValueFromUrl(url, "video_no"))
				if id == 0 {
					continue
				}
				//保存起来
				videos = append(videos, &Video{No: id, Url: fmt.Sprintf("%v%v", "http://www.beautyleg.com/member", url)})
			}
		}
	}

	return albums, videos, nil
}

func getAlbumDetail(url string) ([]*Photo, error) {
	//1. 发起请求
	res, err := httpGet(url, time.Second*time.Duration(30))
	//res, err := readTestFile("/Users/didi/Desktop/Desktop/BEAUTYLEG　腿模 - 2052 Iris.html")
	if err != nil {
		return nil, err
	}

	//2. 解析html
	var photos = make([]*Photo, 0)
	doc := soup.HTMLParse(string(res))
	tbs := doc.FindAll("table")
	for _, tb := range tbs {
		if tb.Attrs()["class"] != "table_all" { //无用的table
			continue
		}

		trs := tb.Find("tr").Find("td").Find("table").FindAll("tr")
		for _, tr := range trs {
			tds := tr.FindAll("td")
			for _, td := range tds {
				url := td.Find("a").Attrs()["href"]
				name := getFileNameFromUrl(url)
				photos = append(photos, &Photo{Name: name, Url: fmt.Sprintf("%v%v", "http://www.beautyleg.com", url)})
			}
		}
	}
	return photos, nil
}

func getVideoDetail(url string, ) (string, string, error) {
	//1. 发起请求
	res, err := httpGet(url, time.Second*time.Duration(30))
	//res, err := readTestFile("/Users/didi/Desktop/Desktop/video.html")
	if err != nil {
		return "", "", err
	}

	//2. 解析html
	doc := soup.HTMLParse(string(res))
	name := doc.Find("center").Text()
	href := doc.Find("center").Find("a").Attrs()["href"]
	return name, fmt.Sprintf("%v/%v", "http://www.beautyleg.com/member", href), nil
}

func getFieldValueFromUrl(url string, fieldName string) string {
	var res string
	s1 := strings.Split(url, "?")
	if len(s1) < 2 {
		return ""
	}
	s2 := strings.Split(s1[1], "&")
	for _, v := range s2 {
		s3 := strings.Split(v, "=")
		if len(s3) < 2 {
			continue
		}
		if fieldName == s3[0] {
			res = s3[1]
			break
		}
	}
	return res
}

func getFileNameFromUrl(url string) string {
	s1 := strings.Split(url, "?")
	s2 := strings.Split(s1[0], "/")
	return s2[len(s2)-1]
}

func httpGet(url string, timeout time.Duration) ([]byte, error) {
	time.Sleep(time.Second) //每次都需要10秒或者以上才能发起请求
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("failed to new request, error: %v\n", err.Error())
		return nil, err
	}

	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36 Edg/92.0.902.73")
	request.Header.Set("Upgrade-Insecure-Requests", "1")
	request.Header.Set("Referer", "http://beautyleg.com/model_list.php")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	if SID != "" {
		request.AddCookie(&http.Cookie{Name: "PHPSESSID", Value: SID})
	}
	//设置用户名和密码
	request.SetBasicAuth("qq375300791", "lmw1234")
	//create client
	cli := &http.Client{
		Timeout: timeout,
	}
	resp, err := cli.Do(request)
	if err != nil {
		fmt.Printf("do request fail, error: %v\n", err.Error())
		return nil, err
	}
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusCreated &&
		resp.StatusCode != http.StatusNoContent {
		fmt.Printf("response status code is not OK, url: %v, status code: %v\n", url, resp.StatusCode)
		return nil, errors.New(fmt.Sprintf("[%v]%s", resp.StatusCode, resp.Status))
	}

	//解析php session id
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "PHPSESSID" {
			SID = cookie.Value
		}
	}

	//4. 解析响应
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("read response fail, url: %v, error: %v\n", url, err.Error())
		return nil, err
	}
	return data, nil
}
func downloadFile(dir, file string, url string, timeout time.Duration) error {
	if err := W.Acquire(context.Background(), 1); err != nil {
		_ = fmt.Errorf("failed to acquire thread, err: %v", err.Error())
		return err
	}

	go func() {
		defer W.Release(1)
		//检查目录是否存在
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			_ = os.MkdirAll(dir, fs.ModePerm)
		}
		//检查文件是否已经存在
		_, err := os.Stat(dir + "/" + file)
		if err == nil || os.IsExist(err) {
			return
		}

		fmt.Printf("download file: %v\n", url)
		res, err := httpGet(url, timeout)
		if err != nil {
			_ = fmt.Errorf("failed to download file, url: %v", url)
			return
		}

		if err := ioutil.WriteFile(dir+"/"+file, res, fs.ModePerm); err != nil {
			_ = fmt.Errorf("failed to save file, path: %v", dir+"/"+file)
			return
		}
	}()
	return nil
}

func readTestFile(file string) ([]byte, error) {
	return ioutil.ReadFile(file)
}
