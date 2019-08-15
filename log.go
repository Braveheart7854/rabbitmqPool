package rabbitmqPool

import (
	"bytes"
	"github.com/pkg/errors"
	"os"
	"log"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

type logger struct {
	File     string
	category string
	level    string
	DataFile string
}

var Logger logger

func init() {
	logFile, _ := ParsePath("./app.log")
	dataFile, _ := ParsePath("./data.log")
	Logger = logger{
		File: logFile,
		DataFile: dataFile,
	}
}

func (logger *logger) write(filePath string,level string, category string, msg ...interface{}) {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			_, err := os.Create(filePath)
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}

	logFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_APPEND, 0666)
	defer logFile.Close()
	if err != nil {
		panic(err)
	}

	// 创建一个日志对象
	l := log.New(logFile, level, log.LstdFlags)

	s := make([]interface{}, 1)
	s[0] = "[" + category + "]"
	msg = append(s, msg)

	l.Println(msg...)
	//logger.category = ""
}

func (logger *logger) Category(category string) *logger {
	logger.category = category
	return logger
}

func (logger *logger) Info(msg ...interface{}) {
	logger.write(logger.File,"[Info]","",msg...)
}

func (logger *logger) Error(msg ...interface{}) {
	logger.write(logger.File,"[Error]","",msg...)
}

func (logger *logger) Notice(category string, msg ...interface{}){
	logger.write(logger.DataFile,"[Notice]",category,msg...)
}


// 解析路径
func ParsePath(path string) (string, error) {
	str := []rune(path)
	firstKey := string(str[:1])

	if firstKey == "~" {
		home, err := home()
		if err != nil {
			return "", err
		}

		return home + string(str[1:]), nil
	} else if firstKey == "." {
		p, _ := filepath.Abs(filepath.Dir(os.Args[0]))
		return p + "/" + path, nil
	} else {
		return path, nil
	}
}


func home() (string, error) {
	u, err := user.Current()
	if nil == err {
		return u.HomeDir, nil
	}

	// cross compile support

	if "windows" == runtime.GOOS {
		return homeWindows()
	}

	// Unix-like system, so just assume Unix
	return homeUnix()
}

func homeUnix() (string, error) {
	// First prefer the HOME environmental variable
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}

	// If that fails, try the shell
	var stdout bytes.Buffer
	cmd := exec.Command("sh", "-c", "eval echo ~$USER")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "", errors.New("blank output when reading home directory")
	}

	return result, nil
}

func homeWindows() (string, error) {
	drive := os.Getenv("HOMEDRIVE")
	path := os.Getenv("HOMEPATH")
	home := drive + path
	if drive == "" || path == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		return "", errors.New("HOMEDRIVE, HOMEPATH, and USERPROFILE are blank")
	}

	return home, nil
}