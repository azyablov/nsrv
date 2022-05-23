package urllist

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

func GetURLList(dir string) ([]string, error) {
	// Considreing all files with .lst suffix
	var urlList []string

	// Getting directory contents
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return urlList, err
	}

	// Looping over FileInfo list
	for _, file := range files {

		// Skip if it's directory
		if file.IsDir() {
			continue
		}

		// Compiling regexp
		fileNameRegexp := regexp.MustCompile(`.*\.lst$`)

		// Check file name against compiled regexp
		matched := fileNameRegexp.MatchString(file.Name())
		if matched {
			urlList = append(urlList, file.Name())
			continue
		}

	}
	return urlList, nil
}

func GetURLContents(dir string, fName string) ([]string, error) {
	bs, err := ioutil.ReadFile(strings.Join([]string{dir, fName}, "/"))
	if err != nil {
		return nil, fmt.Errorf("can't retrieve URL list contents. %v", err)
	}
	urls := strings.Split(string(bs), "\n")
	return urls, nil

}

func UpdateURLList(dir string, fName string, url string) error {
	// TODO: verify pattern for url
	// Trimming spaces...
	url = strings.TrimSpace(url)

	// 1/ Max 255 chars
	if len(url) > 255 {
		return fmt.Errorf("URL is longer than 255 characters")
	}

	// 2/ No http and https at the begginig + no : or //
	reHTTPHeader := regexp.MustCompile(`^http(s)?(:)?/*`)
	HTTPHeader := reHTTPHeader.FindString(url)
	if HTTPHeader != "" {
		return fmt.Errorf("please do not use leading protocol specification http/https: %s", HTTPHeader)
	}

	// 3/ Allowed tocken separators: . /
	// 4/ Prohibited ? & = ; + in domain part
	// 5/ Check for * and ^:
	// 	- only one in tocken!
	// 	- no more than one * in the whole string
	//  - if * is the last, then / should precede - duplicates rules for !!!

	// + check again modified URL pattern = aded ^ and * to allow whildcards
	reDNS := regexp.MustCompile(`^((?:(?:[a-zA-Z0-9]|\*|\^|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)+(?:\*|\^|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9]))(/.*)*`)
	urlSplit := reDNS.FindStringSubmatch(url)
	if urlSplit == nil {
		return fmt.Errorf("URL '%s' is not valid one, please check syntax and whild cards ones again", url)
	}

	if strings.Count(urlSplit[1], "*") > 1 {
		return fmt.Errorf("not more than one * whildcard is allowed in domain part: %s", urlSplit[1])
	}
	log.Println(urlSplit)

	// 6/ Should have / at the end, but do not add /,
	// 	- if last symbol is /
	// 	- or matches pattern / <tocken> + no ^ in training part

	if len(urlSplit[2]) > 0 {
		log.Printf("trailing part: |%s|", urlSplit[2])
		// Checking trailing part for ^
		if strings.Contains(urlSplit[2], "^") {
			return fmt.Errorf("^ in '%s' is not allowed after trailing slash", urlSplit[2])
		}
		// Checking trailing part for *
		if strings.Count(urlSplit[1], "*") > 1 {
			return fmt.Errorf("not more than one * whildcard is allowed in trailing part: %s", urlSplit[1])
		}
		// Normalize before appending
		url = strings.Join([]string{url, "\n"}, "")
	} else {
		// Normalize before appending
		url = strings.Join([]string{url, "\n"}, "/")
	}

	// Open file to append URL string
	fh, err := os.OpenFile(strings.Join([]string{dir, fName}, "/"), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer fh.Close()

	// Appending URL
	if _, err := fh.Write([]byte(url)); err != nil {
		return err
	}

	return nil
}
