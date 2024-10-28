package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/yann0917/dedao-dl/cmd/app"
	"github.com/yann0917/dedao-dl/services"
	"github.com/yann0917/dedao-dl/utils"
)

var articleCmd = &cobra.Command{
	Use:     "article",
	Short:   "获取文章详情",
	Long:    `使用 dedao-dl article 获取文章详情`,
	Args:    cobra.NoArgs,
	PreRunE: AuthFunc,
	RunE: func(cmd *cobra.Command, args []string) error {
		if classID > 0 && articleID == 0 {
			if err := articleList(classID); err != nil {
				return err
			}
		}

		if articleID > 0 {
			err := articleDetail(classID, articleID)
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(articleCmd)
	articleCmd.PersistentFlags().IntVarP(&classID, "id", "i", 0, "课程id")
	articleCmd.PersistentFlags().IntVarP(&articleID, "aid", "a", 0, "文章id")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// testCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
}

func articleList(id int) (err error) {
	list, err := app.ArticleList(id, "")
	if err != nil {
		return
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"#", "ID", "课程名称", "更新时间", "音频进度", "是否阅读"})
	table.SetAutoWrapText(false)

	for i, p := range list.List {
		isRead := "❌"
		if p.IsRead {
			isRead = "✔"
		}
		listenProgress := "0"
		if p.Audio != nil {
			listenProgress = strconv.FormatFloat(p.Audio.ListenProgress, 'g', 5, 32)
		}
		table.Append([]string{strconv.Itoa(i),
			p.IDStr, p.Title,
			utils.Unix2String(int64(p.UpdateTime)),
			listenProgress,
			isRead,
		})
	}
	table.Render()
	return
}

func articleDetail(id, aid int) (err error) {
	detail, _, err := app.ArticleDetail(id, aid)

	if err != nil {
		return
	}
	out := os.Stdout
	table := tablewriter.NewWriter(out)

	var content []services.Content
	jsoniter.UnmarshalFromString(detail.Content, &content)
	fmt.Fprint(out, contentsToMarkdown(content))
	fmt.Fprintln(out)
	table.Render()
	return
}

func contentsToMarkdown(contents []services.Content) (res string) {
	for _, content := range contents {
		switch content.Type {
		case "audio":
			title := strings.TrimRight(content.Title, ".mp3")
			res += getMdHeader(1) + title + "\r\n\r\n"
		case "header":
			res += getMdHeader(content.Level) + content.Text + "\r\n\r\n"
		case "blockquote":
			texts := strings.Split(content.Text, "\n")
			for _, text := range texts {
				res += "> " + text + "\r\n"
				res += "> \r\n"
			}
			res = strings.TrimRight(res, "> \r\n")
			res += "\r\n\r\n"
		case "paragraph":
			// map 转结构体
			tmpJson, err := jsoniter.Marshal(content.Contents)
			if err != nil {
				return
			}
			cont := services.Contents{}
			err = jsoniter.Unmarshal(tmpJson, &cont)
			if err != nil {
				return ""
			}
			for _, item := range cont {
				subContent := strings.Trim(item.Text.Content, " ")
				switch item.Type {
				case "text":
					if item.Text.Bold {
						res += " **" + subContent + "** "
					} else if item.Text.Highlight {
						res += " *" + subContent + "* "
					} else {
						res += subContent
					}
				}
			}
			res = strings.Trim(res, " ")
			res = strings.Trim(res, "\r\n")
			res += "\r\n\r\n"
		case "list":
			tmpJson, err := jsoniter.Marshal(content.Contents)
			if err != nil {
				return
			}
			var cont []services.Contents
			err = jsoniter.Unmarshal(tmpJson, &cont)
			if err != nil {
				return ""
			}

			for _, item := range cont {
				for _, item := range item {
					subContent := strings.Trim(item.Text.Content, " ")
					switch item.Type {
					case "text":
						if item.Text.Bold {
							res += "* **" + subContent + "** "
						} else if item.Text.Highlight {
							res += "* *" + subContent + "* "
						} else {
							res += "* " + subContent
						}
					}
				}
				res += "\r\n\r\n"
			}
		case "elite": // 划重点
			res += getMdHeader(2) + "划重点\r\n\r\n" + content.Text + "\r\n\r\n"

		case "image":
			res += "![" + content.URL + "](" + content.URL + ")" + "\r\n\r\n"
		case "label-group":
			res += getMdHeader(2) + "`" + content.Text + "`" + "\r\n\r\n"
		}
	}

	res += "---\r\n"
	return
}

func articleCommentsToMarkdown(contents []services.ArticleComment) (res string) {
	res = getMdHeader(2) + "热门留言\r\n\r\n"
	for _, content := range contents {
		res += content.NotesOwner.Name + "：" + content.Note + "\r\n\r\n"
		if content.CommentReply != "" {
			res += "> " + content.CommentReplyUser.Name + "(" + content.CommentReplyUser.Role + ") 回复：" + content.CommentReply + "\r\n\r\n"
		}
	}
	res += "---\r\n"
	return
}

func getMdHeader(level int) string {
	switch level {
	case 1:
		return "# "
	case 2:
		return "## "
	case 3:
		return "### "
	case 4:
		return "#### "
	case 5:
		return "##### "
	case 6:
		return "###### "
	default:
		return ""
	}
}

func DownloadMarkdown(cType string, id int, path string) error {
	switch cType {
	case app.CateCourse:
		list, err := app.ArticleList(id, "")
		if err != nil {
			return err
		}
		for _, v := range list.List {
			detail, _, err := app.ArticleDetail(id, v.ID)
			if err != nil {
				fmt.Println(err.Error())
				return err
			}

			var content []services.Content
			err = jsoniter.UnmarshalFromString(detail.Content, &content)
			if err != nil {
				return err
			}

			name := utils.FileName(strconv.Itoa(detail.Article.CreateTime)+"@"+v.Title, "md")
			fileName := filepath.Join(path, name)
			fmt.Printf("正在生成文件：【\033[37;1m%s\033[0m】 ", name)
			_, exist, err := utils.FileSize(fileName)
			if err != nil {
				fmt.Printf("\033[31;1m%s\033[0m\n", "失败"+err.Error())
				return err
			}

			if exist {
				fmt.Printf("\033[33;1m%s\033[0m\n", "已存在")
				return nil
			}

			res := contentsToMarkdown(content)
			// 添加留言 -- 暂时取消
			// commentList, err := app.ArticleCommentList(enId, "like", 1, 20)
			// if err == nil {
			// 	res += articleCommentsToMarkdown(commentList.List)
			// }

			f, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Printf("\033[31;1m%s\033[0m\n", "失败"+err.Error())
				return err
			}
			_, err = f.WriteString(res)
			if err != nil {
				fmt.Printf("\033[31;1m%s\033[0m\n", "失败"+err.Error())
				return err
			}
			if err = f.Close(); err != nil {
				if err != nil {
					return err
				}
			}
			fmt.Printf("\033[32;1m%s\033[0m\n", "完成")
		}
	case app.CateAudioBook:

	}

	return nil

}
