package main

// 原始代码 https://studygolang.com/articles/31251

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unsafe"

	hq "github.com/antchfx/htmlquery"
)

const (
	UserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.75 Safari/537.36"
	BingHomeURL    = "https://cn.bing.com"
	WallPaperURL   = "https://bing.ioliu.cn/?p=1"
	CurrentPathDir = "images/"
)

// ImageSize 图片大小
type ImageSize struct {
	Name          string
	Width, Height int
}

var (
	Size0k = ImageSize{"0k",  640,  480}
	Size1k = ImageSize{"1k", 1920, 1080}
	Size2k = ImageSize{"2k", 2560, 1440}
	Size4k = ImageSize{"4k", 3840, 2160}
)

type WallPaper struct {
	Image, Title, Date string
}

func init() {
	_ = os.Mkdir(CurrentPathDir, 0755)
	flag.Parse()
}

func main() {
	var imageURL, imagePath, savePath string
	if args := flag.Args(); len(args) > 0 {
		dayTarget := args[0]
		if len(dayTarget) >= 8 {
			dayTarget = dayTarget[:8]
		} else {
			dayTarget = time.Now().Format("20060102")
		}
		savePath = GetWallPaperImage(dayTarget)
		if savePath == "" {
			fmt.Println("没有找到当天的背景图")
			return
		}
	} else {
		imageURL, imagePath = GetBingTodayImage()
		if imageURL == "" || imagePath == "" {
			fmt.Println("没有找到当天的背景图")
			return
		}
		savePath, _ = SaveImage(imageURL, imagePath)
	}

	fmt.Println("设置桌面...")
	err := SetWindowsWallpaper(savePath)
	if err != nil {
		fmt.Println("设置桌面背景失败: " + err.Error())
		return
	}
}

// EncodeMD5 MD5编码
func EncodeMD5(value string) string {
	m := md5.New()
	m.Write([]byte(value))
	return hex.EncodeToString(m.Sum(nil))
}

// SetWindowsWallpaper 设置windows壁纸
func SetWindowsWallpaper(imagePath string) error {
	dll := syscall.NewLazyDLL("user32.dll")
	proc := dll.NewProc("SystemParametersInfoW")
	_t, _ := syscall.UTF16PtrFromString(imagePath)
	ret, _, _ := proc.Call(20, 1, uintptr(unsafe.Pointer(_t)), 0x1|0x2)
	if ret != 1 {
		return errors.New("系统调用失败")
	}
	return nil
}

// SaveImage 找到已保存的图片或下载保存图片
func SaveImage(imageURL, imagePath string) (string, error) {
	savePath, err := filepath.Abs(imagePath)
	if err != nil {
		fmt.Println("找不到路径: " + err.Error())
		return "", err
	}

	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		fmt.Printf("下载图片... %s\n", imageURL)
		if err = DownloadImage(imageURL, imagePath); err != nil {
			fmt.Println("下载图片失败: " + err.Error())
			return "", err
		}
	} else {
		fmt.Println("图片已存在: " + savePath)
	}

	return savePath, err
}

// DownloadImage 下载图片,保存并返回保存的文件名的绝对路径
func DownloadImage(imageURL, imagePath string) (err error) {
	client := http.Client{}
	request, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return
	}

	response, err := client.Do(request)
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	err = ioutil.WriteFile(imagePath, body, 0755)
	return
}

// GetImageSrc 解析为高清大图的地址
func GetImageSrc(imgSrc string, oldSize, newSize ImageSize) string {
	if imgSrc = strings.TrimSpace(imgSrc); imgSrc == "" {
		return imgSrc
	}
	oldStr := fmt.Sprintf("%dx%d", oldSize.Width, oldSize.Height)
	newStr := fmt.Sprintf("%dx%d", newSize.Width, newSize.Height)
	imgSrc = imgSrc[:strings.LastIndex(imgSrc, "?")]
	return strings.Replace(imgSrc, oldStr, newStr, 1)
}

// GetWallPaperList 获取Bing背景图列表
func GetWallPaperList(page int) (wpList map[string]WallPaper) {
	wpList = make(map[string]WallPaper, 0)
	wpURL := WallPaperURL
	if page > 1 {
		wpURL = strings.Replace(wpURL, "p=1", fmt.Sprintf("p=%d", page), 1)
	}

	fmt.Println("获取列表中...")
	request, err := http.NewRequest("GET", wpURL, nil)
	if err != nil {
		return
	}
	request.Header.Set("user-agent", UserAgent)
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return
	}
	htmlDoc, err := hq.Parse(response.Body)
	if err != nil {
		return
	}

	divCards, err := hq.QueryAll(htmlDoc, "//div[@class=\"card progressive\"]")
	for _, card := range divCards {
		imgSrc := hq.SelectAttr(hq.FindOne(card, "//img/@src"), "src")
		wp := WallPaper{Image: GetImageSrc(imgSrc, Size0k, Size1k)}
		desc := hq.FindOne(card, "//div[@class=\"description\"]")
		wp.Title = hq.InnerText(hq.FindOne(desc, "//h3"))
		cal := hq.FindOne(desc, "//p[@class=\"calendar\"]")
		wp.Date = hq.InnerText(hq.FindOne(cal, "//em"))
		wpList[strings.ReplaceAll(wp.Date, "-", "")] = wp
	}

	return
}

// GetWallPaperImage 获取Bing以往的背景图
func GetWallPaperImage(dayTarget string) string {
	var wpList map[string]WallPaper
	for i := 1; i <= 10; i++ {
		wpList = GetWallPaperList(i)
		for dayName, wp := range wpList {
			imageURL, imagePath := GetSavePath(dayName, wp.Image, Size1k, false)
			if imageURL == "" || imagePath == "" {
				continue
			}
			savePath, _ := SaveImage(imageURL, imagePath)
			if dayName == dayTarget {
				return savePath
			}
		}
	}
	return ""
}

// GetBingTodayImage 获取Bing今天的背景图
func GetBingTodayImage() (string, string) {
	fmt.Println("获取必应背景图中...")
	imageURL, err := GetBingBackgroundImageURL()
	if err != nil {
		fmt.Println("获取背景图片链接失败: " + err.Error())
		return "", ""
	}
	fmt.Println("成功: " + imageURL)

	dayName := time.Now().Format("20060102")
	return GetSavePath(dayName, imageURL, Size4k, true)
}

// GetBingBackgroundImageURL 获取bing主页的背景图片链接
func GetBingBackgroundImageURL() (string, error) {
	client := http.Client{}

	request, err := http.NewRequest("GET", BingHomeURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("user-agent", UserAgent)

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	htmlDoc, err := hq.Parse(response.Body)
	if err != nil {
		return "", err
	}

	item := hq.FindOne(htmlDoc, "//div[@id=\"bgImgProgLoad\"]")
	result := hq.SelectAttr(item, "data-ultra-definition-src")
	return BingHomeURL + result, nil
}

// GetSavePath 下载图片,保存并返回保存的文件名的绝对路径
func GetSavePath(dayName, imageURL string, size ImageSize, replace bool) (string, string) {
	if replace {
		wRegexp := regexp.MustCompile("w=\\d+")
		hRegexp := regexp.MustCompile("h=\\d+")
		imageURL = wRegexp.ReplaceAllString(imageURL, fmt.Sprintf("w=%d", size.Width))
		imageURL = hRegexp.ReplaceAllString(imageURL, fmt.Sprintf("h=%d", size.Height))
	}

	var sizeName string
	if sizeName = size.Name; sizeName == "" {
		sizeName = fmt.Sprintf("%04d%04d", size.Width, size.Height)
	}
	hashName := EncodeMD5(imageURL)[24:]
	fileName := fmt.Sprintf("%s-%s-%s.%s", dayName, sizeName, hashName, "jpg")
	imagePath := strings.TrimRight(CurrentPathDir, "/") + "/" + fileName

	return imageURL, imagePath
}
