package tpl2entity

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/generaterepository/pkg"
)

const (
	EOF                  = "\n"
	WINDOW_EOF           = "\r\n"
	HTTP_HEAD_BODY_DELIM = EOF + EOF
)

const (
	TPL_DEFINE_TYPE_CURL_REQUEST  = "curl_request"
	TPL_DEFINE_TYPE_CURL_RESPONSE = "curl_response"
	TPL_DEFINE_TYPE_SQL_SELECT    = "sql_select"
	TPL_DEFINE_TYPE_SQL_UPDATE    = "sql_update"
	TPL_DEFINE_TYPE_SQL_INSERT    = "sql_insert"
	TPL_DEFINE_TYPE_TEXT          = "text"
	CHARACTERISTIC_CURL           = "HTTP/1.1"
	CHARACTERISTIC_SQL_SELECT     = "SELECT"
	CHARACTERISTIC_SQL_UPDATE     = "UPDATE"
	CHARACTERISTIC_SQL_INSERT     = "INSERT"
	SQL_VARIABLE_DELIM            = ':'
)

var (
	LeftDelim  = "{{"
	RightDelim = "}}"
)

type TPLDefines []*TPLDefine

type TPLDefine struct {
	Name      string
	Namespace string
	Text      string // 模板执行后的输出(gqt  需要使用)
	Content   string // 不包含 {{define }} 和{{end}}
	typ       string
}

//ParseDefine 解析模板内Define
func ParseDefine(tpl string, namespace string) (tplDefines TPLDefines, err error) {
	// 解析文本
	delim := LeftDelim + "define "
	delimLen := len(delim)
	tpl = pkg.TrimSpaces(tpl) // 去除开头结尾的非有效字符
	defineList := make([]string, 0)
	for {
		index := strings.Index(tpl, delim)
		if index >= 0 {
			pos := delimLen + index
			nextIndex := strings.Index(tpl[pos:], delim)
			if nextIndex >= 0 {
				sepPos := pos + nextIndex
				oneDefine := tpl[:sepPos]
				defineList = append(defineList, oneDefine)
				tpl = tpl[sepPos:]
			} else {
				defineList = append(defineList, tpl)
				break
			}
		} else {
			break
		}
	}

	tplDefines = TPLDefines{}

	// 格式化
	for _, defineText := range defineList {
		tplDefine, err := NewTPLDefine(defineText, namespace)
		if err != nil {
			return nil, err
		}
		tplDefines = append(tplDefines, tplDefine)
	}

	return
}

func NewTPLDefine(defineText string, namespace string) (tplDefine *TPLDefine, err error) {
	tplDefine = &TPLDefine{
		Namespace: namespace,
		Text:      defineText,
	}
	err = tplDefine.parseContent()
	if err != nil {
		return nil, err
	}
	err = tplDefine.parseName()
	if err != nil {
		return nil, err
	}

	return

}

func (d *TPLDefine) GetVairables() (variables Variables) {
	content := []byte(d.Content)
	switch d.typ {
	case TPL_DEFINE_TYPE_SQL_SELECT, TPL_DEFINE_TYPE_SQL_UPDATE, TPL_DEFINE_TYPE_SQL_INSERT:
		return parsSqlTplVariable(content)
	}

	subVariables := parseTplVariable(content)
	variables = append(variables, subVariables...)
	byteArr := []byte(content)

	// parse sql variable
	for {
		variable, pos := parsePrefixVariable(byteArr, SQL_VARIABLE_DELIM)
		if variable.Name == "" {
			break
		}
		variable.FieldName = variable.Name
		variables = append(variables, &variable)
		pos += len(variable.Name)
		byteArr = byteArr[pos:]
	}
	variables = variables.UniqueItems()
	return variables
}

func (d *TPLDefine) Fullname() (fullname string) {
	fullname = fmt.Sprintf("%s.%s", d.Namespace, d.Name)
	return
}
func (d *TPLDefine) FullnameCamel() (fullnameCamel string) {
	fullname := fmt.Sprintf("%s_%s", strings.ReplaceAll(d.Namespace, ".", "_"), d.Name)
	fullnameCamel = pkg.ToCamel(fullname)
	return
}

func (d *TPLDefine) parseName() (err error) {
	delim := []byte(fmt.Sprintf(`%sdefine "`, LeftDelim))
	tplDefineByte := []byte(d.Text)
	index := bytes.Index(tplDefineByte, delim)
	nameByte := make([]byte, 0)
	if index >= 0 {
		index += len(delim)
		for i := index; i < len(tplDefineByte); i++ {
			c := tplDefineByte[i]
			if c != '"' {
				nameByte = append(nameByte, tplDefineByte[i])
			} else {
				break
			}

		}
	}
	defineName := string(nameByte)
	if defineName == "" {
		err = errors.Errorf("define name is empty")
		return err
	}
	d.Name = defineName
	return nil
}

// Content TPLDefine.Out 含有{{define }} ...{{end}} Content 在此基础上 去除 define标记
func (d *TPLDefine) parseContent() (err error) {
	content := pkg.TrimSpaces(d.Text) // 去除开头结尾的非有效字符
	index := strings.Index(content, RightDelim)
	if index < 0 {
		err = errors.Errorf("not found %sdefine \"xxx\" %s in tpl content %s", LeftDelim, RightDelim, content)
		return err
	}

	endTag := fmt.Sprintf("%send%s", LeftDelim, RightDelim)
	endIndex := strings.Index(content, endTag)
	if endIndex < 0 {
		err = errors.Errorf("not found %send%s in tpl content %s", LeftDelim, RightDelim, content)
		return err
	}
	content = content[index+len(RightDelim) : len(content)-len(endTag)]
	content = pkg.TrimSpaces(content)
	d.Content = content
	return nil
}

func (d *TPLDefine) ContentFirstLine(s string) (firstLine string) {
	re, err := regexp.Compile(fmt.Sprintf("%s.*%s", LeftDelim, RightDelim))
	if err != nil {
		panic(err)
	}
	for {
		firstLineIndex := strings.Index(s, EOF)
		if firstLineIndex < 0 {
			firstLine = s
			break
		}
		firstLine = s[:firstLineIndex]
		firstLine = re.ReplaceAllString(firstLine, "") // 删除template 模板变量，防止第一行为模板变量行，如果为模板变量则取下一行
		firstLine = pkg.TrimSpaces(firstLine)
		if firstLine != "" {
			break
		}
		s = s[firstLineIndex+len(EOF):] // 更新s 再次获取
	}
	return
}

func (d *TPLDefine) TypeTitle() (title string) {
	typ := d.Type()
	mapConfig := map[string]string{
		TPL_DEFINE_TYPE_CURL_REQUEST:  "请求",
		TPL_DEFINE_TYPE_CURL_RESPONSE: "响应",
		TPL_DEFINE_TYPE_SQL_SELECT:    "查询",
		TPL_DEFINE_TYPE_SQL_UPDATE:    "更新",
		TPL_DEFINE_TYPE_SQL_INSERT:    "新增",
	}
	return mapConfig[typ]
}

// Type 判断 tpl define 属于那种类型
func (d *TPLDefine) Type() (typ string) {
	if d.typ != "" {
		return d.typ
	}
	typ = TPL_DEFINE_TYPE_TEXT
	content := d.Content
	firstLine := d.ContentFirstLine(content)
	if firstLine == "" {
		d.typ = typ
		return
	}
	curlLen := len(CHARACTERISTIC_CURL)
	fl := len(firstLine)
	if fl >= curlLen {
		last := strings.ToUpper(firstLine[fl-curlLen:])
		if last == CHARACTERISTIC_CURL {
			typ = TPL_DEFINE_TYPE_CURL_REQUEST
			d.typ = typ
			return
		}

		first := strings.ToUpper(firstLine[:curlLen])
		if first == CHARACTERISTIC_CURL {
			typ = TPL_DEFINE_TYPE_CURL_RESPONSE
			d.typ = typ
			return
		}
	}

	sqlSelectLen := len(CHARACTERISTIC_SQL_SELECT)
	if fl >= sqlSelectLen {
		first := strings.ToUpper(firstLine[:sqlSelectLen])
		if first == CHARACTERISTIC_SQL_SELECT {
			typ = TPL_DEFINE_TYPE_SQL_SELECT
			d.typ = typ
			return
		}
	}

	sqlUpdateLen := len(CHARACTERISTIC_SQL_UPDATE)
	if fl > sqlUpdateLen {
		first := strings.ToUpper(firstLine[:sqlUpdateLen])
		if first == CHARACTERISTIC_SQL_UPDATE {
			typ = TPL_DEFINE_TYPE_SQL_UPDATE
			d.typ = typ
			return
		}
	}

	sqlInsertLen := len(CHARACTERISTIC_SQL_INSERT)
	if fl > sqlInsertLen {
		first := strings.ToUpper(firstLine[:sqlInsertLen])
		if first == CHARACTERISTIC_SQL_INSERT {
			typ = TPL_DEFINE_TYPE_SQL_INSERT
			d.typ = typ
			return
		}
	}

	d.typ = typ
	return
}

// 判断是否为CURL 类型
func (d *TPLDefine) ISCURL() (yes bool) {
	typ := d.Type()
	yes = (typ == TPL_DEFINE_TYPE_CURL_REQUEST) || (typ == TPL_DEFINE_TYPE_CURL_RESPONSE)
	return
}

// 判断是否为SQL 类型
func (d *TPLDefine) ISSQL() (yes bool) {
	typ := d.Type()
	yes = (typ == TPL_DEFINE_TYPE_SQL_SELECT) || (typ == TPL_DEFINE_TYPE_SQL_UPDATE) || (typ == TPL_DEFINE_TYPE_SQL_INSERT)
	return
}

// Tag TPLDefine 标签 namespace 的后缀（curl、sql、ddl、meta）
func (d *TPLDefine) Tag() (tag string) {
	lastIndex := strings.Index(d.Namespace, ".")
	tag = d.Namespace
	if lastIndex > -1 {
		tag = d.Namespace[lastIndex+1:]
	}
	return
}

// 判断一个(变量)名词是否和define 名称相同
func (dl TPLDefines) IsDefineNameCamel(variableName string) bool {
	for _, TPLDefine := range dl {
		if pkg.ToCamel(TPLDefine.Name) == variableName {
			return true
		}
	}
	return false
}

// 去重，保留第一个出现的值，维持原有顺序
func (dl TPLDefines) UniqueItems() (uniq TPLDefines) {
	vmap := make(map[string]*TPLDefine)
	uniq = TPLDefines{}
	for _, tplDefine := range dl {
		if _, ok := vmap[tplDefine.Name]; ok {
			continue
		} else {
			vmap[tplDefine.Name] = tplDefine
			uniq = append(uniq, tplDefine)
		}
	}
	return
}
