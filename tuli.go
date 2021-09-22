package main

import (
	"fmt"
	"github.com/anaskhan96/soup"
	"strings"
	"time"
)

type Category struct {
	Name   string
	Url    string
	Albums []*Album
}

type Pager struct {
	Urls []string
}

func grabTuliMainPage(url string, dir string) ([]*Category, error) {
	//1. 获取所有的category
	categories, err := getTuliCategories(url)
	if err != nil {
		fmt.Printf("failed to get tuli categories, error: %v\n", err.Error())
		return nil, err
	}

	//2. 对每一个category都抓取详情
	for _, c := range categories {
		var next = c.Url
		for next != "" {
			var as = make([]*Album, 0)
			as, next, err = getTuliPageDetail(next)
			if err != nil { //抓取失败了，跳转到下一个
				continue
			}
			//3. 对每一个album抓取detail
			for _, a := range as {
				var nextDetailPage = a.Url
				for nextDetailPage != "" {
					var photos = make([]*Photo, 0)
					photos, nextDetailPage, err = getTuliPhotoList(nextDetailPage, len(a.Photos))
					if err != nil {
						continue
					}
					if nextDetailPage == "" {
						break
					}
					//更新nextDetailPge
					nextDetailPage = fmt.Sprintf("%v/%v", c.Url, nextDetailPage)
					a.Photos = append(a.Photos, photos...)
				}

				//下载文件
				storeDir := fmt.Sprintf("%v/%v/%v", dir, c.Name, a.Name)
				_ = downloadFile(storeDir, "0000.jpg", a.Cover, time.Second*3)
				for _, p := range a.Photos {
					_ = downloadFile(storeDir, p.Name, p.Url, time.Second*3)
				}
			}

			if next == "" {
				break
			}

			next = fmt.Sprintf("%v/%v/%v", c.Url, c.Name, next)
			c.Albums = append(c.Albums, as...)
		}
	}
	return categories, nil
}

func getTuliCategories(url string) ([]*Category, error) {
	//1. 先下载所有的 category列表
	page, err := httpGet(url, time.Second*3, 0)
	if err != nil {
		fmt.Printf("failed to get main page, url: %v, error: %v\n", url, err.Error())
		return nil, err
	}

	//2. 解析main page
	root := soup.HTMLParse(string(page))
	lis := root.FindStrict("div", "class", "nav_header").FindAll("li")

	//3. 生成category
	res := make([]*Category, 0)
	for i, li := range lis {
		if i == 0 { //跳过首页
			continue
		}
		res = append(res, &Category{
			Name:   li.Find("span").Text(),
			Url:    fmt.Sprintf("%v%v", url, li.Find("a").Attrs()["href"]),
			Albums: make([]*Album, 0),
		})
	}

	return res, nil
}

func getTuliPageDetail(url string) ([]*Album, string, error) {
	//1. 请求详情页
	page, err := httpGet(url, time.Second*3, 0)
	if err != nil {
		fmt.Printf("failed to get category page, url: %v, error: %v\n", url, err.Error())
		return nil, "", err
	}

	//2. 解析main page
	root := soup.HTMLParse(string(page))
	albums := root.Find("div", "id", "container").FindAll("div")
	as := make([]*Album, 0)
	for _, a := range albums {
		if !strings.Contains(a.Attrs()["class"], "post") {
			continue
		}
		url := a.Find("a").Attrs()["href"]
		name := a.Find("a").Attrs()["title"]
		cover := a.Find("a").Find("img").Attrs()["src"]
		as = append(as, &Album{
			Name:   name,
			Url:    fmt.Sprintf("https://www.tuli.cc%v", url),
			Cover:  cover,
			Photos: make([]*Photo, 0),
		})
	}

	//3. 解析pager
	var nextPage string
	lis := root.Find("div", "id", "pager").Find("ul").FindAll("li")
	for _, li := range lis {
		doca := li.Find("a")
		if doca.Pointer == nil {
			continue
		}
		t := doca.Text()
		if t == "下一页" {
			nextPage = li.Find("a").Attrs()["href"]
			break
		}
	}

	if strings.Contains(nextPage, "#") {
		nextPage = ""
	}
	return as, nextPage, nil
}

func getTuliPhotoList(url string, min int) ([]*Photo, string, error) {
	page, err := httpGet(url, time.Second*3, 0)
	if err != nil {
		fmt.Printf("failed to get category page, url: %v, error: %v\n", url, err.Error())
		return nil, "", err
	}

	//2. 解析main page
	root := soup.HTMLParse(string(page))
	photos := make([]*Photo, 0)
	imgs := root.Find("p", "class", "bodyintroduce").FindAll("img")
	if len(imgs) == 0 {
		imgs = root.Find("div", "id", "postarea").FindAll("img")
	}
	for i, img := range imgs {
		url := img.Attrs()["src"]
		photos = append(photos, &Photo{
			Name: fmt.Sprintf("%04d.jpg", min+i+1),
			Url:  url,
		})
	}

	var nextPage string
	lis := root.Find("div", "class", "pageart").Find("ul").FindAll("li")
	for _, li := range lis {
		doca := li.Find("a")
		if doca.Pointer == nil {
			continue
		}
		t := doca.Text()
		if t == "下一页" {
			nextPage = li.Find("a").Attrs()["href"]
			break
		}
	}

	if strings.Contains(nextPage, "#") {
		nextPage = ""
	}

	return photos, nextPage, nil
}
