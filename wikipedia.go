package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/hoisie/web"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Status struct {
	Events []Event `json:"events"`
}

type Event struct {
	Id      int      `json:"event_id"`
	Message *Message `json:"message"`
}

type Message struct {
	Id              string `json:"id"`
	Room            string `json:"room"`
	PublicSessionId string `json:"public_session_id"`
	IconUrl         string `json:"icon_url"`
	Type            string `json:"type"`
	SpeakerId       string `json:"speaker_id"`
	Nickname        string `json:"nickname"`
	Text            string `json:"text"`
}

func defaultAddr() string {
	port := os.Getenv("PORT")
	if port == "" {
		return ":80"
	}
	return ":" + port
}

var addr = flag.String("addr", defaultAddr(), "server address")

var reToken = regexp.MustCompile(`^wp:(.+)$`)

var re = regexp.MustCompile("^([^0-9\\s\\[][^\\s\\[]*)?(\\[[0-9]+\\])?$")

func jsonScan(v interface{}, jp string, t interface{}) (err error) {
	if jp == "" {
		return errors.New("invalid path")
	}
	var ok bool
	for _, token := range strings.Split(jp, "/") {
		sl := re.FindAllStringSubmatch(token, -1)
		if len(sl) == 0 {
			return errors.New("invalid path")
		}
		ss := sl[0]
		if ss[1] != "" {
			if v, ok = v.(map[string]interface{})[ss[1]]; !ok {
				return errors.New("invalid path: " + ss[1])
			}
		}
		if ss[2] != "" {
			i, err := strconv.Atoi(ss[2][1 : len(ss[2])-1])
			if err != nil {
				return errors.New("invalid path: " + ss[2])
			}
			var vl []interface{}
			if vl, ok = v.([]interface{}); !ok {
				if vm, ok := v.(map[string]interface{}); ok {
					for _, vv := range vm {
						v = vv
						break
					}
				} else {
					return errors.New("invalid path: " + ss[2])
				}
			} else {
				if i < 0 || i > len(vl)-1 {
					return errors.New("invalid path: " + ss[2])
				}
				v = vl[i]
			}
		}
	}
	rt := reflect.ValueOf(t).Elem()
	rv := reflect.ValueOf(v)
	defer func() {
		if errstr := recover(); errstr != nil {
			err = errors.New(errstr.(string))
		}
	}()
	rt.Set(rv)
	return nil
}

func main() {
	flag.Parse()

	web.Get("/", func() string {
		return ""
	})
	web.Post("/lingr", func(ctx *web.Context) string {
		var status Status
		err := json.NewDecoder(ctx.Request.Body).Decode(&status)
		ret := ""
		if err == nil && len(status.Events) > 0 {
			text := status.Events[0].Message.Text
			tokens := reToken.FindStringSubmatch(text)
			if len(tokens) == 2 {
				q := "action=query" +
					"&titles=" + url.QueryEscape(tokens[1]) +
					"&prop=revisions" +
					"&rvprop=content" +
					"&redirects=1" +
					"&format=json"

				r, err := http.Get("http://ja.wikipedia.org/w/api.php?" + q)
				if err != nil {
					return "No pedia"
				}
				defer r.Body.Close()
				if r.StatusCode != 200 {
					return "No pedia"
				}
				var v interface{}
				err = json.NewDecoder(r.Body).Decode(&v)
				if err != nil {
					return "No pedia"
				}

				var title, content string
				err = jsonScan(v, "/query/pages[0]/title", &title)
				if err != nil {
					return "No pedia"
				}
				err = jsonScan(v, "/query/pages[0]/revisions[0]/*", &content)
				if err != nil {
					return "No pedia"
				}

				if lines := strings.Split(content, "\n"); len(lines) > 0 {
					content = ""
					for _, line := range lines {
						if len(line) > 3 && line[:3] == "'''" && strings.Contains(line, "''' ") {
							content = line
							break
						}
					}
					if content == "" {
						//content = strings.Join(lines, "\n")
						content = lines[0]
					}
				}
				content = regexp.MustCompile("\\[\\[.+?]]").ReplaceAllStringFunc(content, func(a string) string {
					return strings.Split(a[2:len(a)-2], "|")[0]
				})
				content = regexp.MustCompile("'''.+?'''").ReplaceAllStringFunc(content, func(a string) string {
					return a[3 : len(a)-3]
				})
				content = regexp.MustCompile("''.+?''").ReplaceAllStringFunc(content, func(a string) string {
					return a[2 : len(a)-2]
				})
				content = regexp.MustCompile("{{aimai}}").ReplaceAllStringFunc(content, func(a string) string {
					return "この単語は曖昧過ぎます"
				})
				content = regexp.MustCompile("{{.+?}}").ReplaceAllStringFunc(content, func(a string) string {
					return strings.Split(a[2:len(a)-2], "|")[0]
				})
				content = regexp.MustCompile("<[^>]+?>(?:.|\\n)+?</[^>]+?>").ReplaceAllStringFunc(content, func(a string) string {
					return "\n...\n"
				})
				ret = fmt.Sprintf("http://ja.wikipedia.org/wiki/%s", url.QueryEscape(title)) + "\n" + content
			}
		}
		if len(ret) > 0 {
			ret = strings.TrimRight(ret, "\n")
			if runes := []rune(ret); len(runes) > 1000 {
				ret = string(runes[0:999])
			}
		}
		return ret
	})
	web.Run(*addr)
}
