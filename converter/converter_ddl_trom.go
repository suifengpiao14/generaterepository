package converter

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/suifengpiao14/gotemplatefunc/templatefunc"
	"github.com/suifengpiao14/tormrepository/pkg/ddlparser"
)

var TormTemplatefuncMap = template.FuncMap{
	"zeroTime":      templatefunc.ZeroTime,
	"currentTime":   templatefunc.CurrentTime,
	"permanentTime": templatefunc.PermanentTime,
	"contains":      strings.Contains,
	"toCamel":       templatefunc.ToCamel,
	"toLowerCamel":  templatefunc.ToLowerCamel,
	"snakeCase":     templatefunc.SnakeCase,

	"tplGetByPrimaryKey":        tplGetByPrimaryKey,
	"tplGetAllByPrimaryKeyList": tplGetAllByPrimaryKeyList,
	"tplPaginateWhere":          tplPaginateWhere,
	"tplPaginateTotal":          tplPaginateTotal,
	"tplPaginate":               tplPaginate,
	"tplInsert":                 tplInsert,
	"tplUpdate":                 tplUpdate,
	"tplDel":                    tplDel,
}

type TormDTO struct {
	Name string
	TPL  string
}

type TormDTOs []*TormDTO

//GenerateTorm  生成torm文件内容
func GenerateTorm(tormTpl *template.Template, tableList []*ddlparser.Table) (tormDTOs TormDTOs, err error) {
	tormDTOs = TormDTOs{}
	for _, table := range tableList {
		var buf bytes.Buffer
		err = tormTpl.Execute(&buf, table)
		if err != nil {
			return nil, err
		}
		tormDTO := TormDTO{
			Name: table.TableNameCamel(),
			TPL:  buf.String(),
		}
		tormDTOs = append(tormDTOs, &tormDTO)
	}

	return tormDTOs, nil
}

func tplGetAllByPrimaryKeyList(table *ddlparser.Table) (tpl string, err error) {
	primaryKeyCamel := table.PrimaryKeyCamel()
	prefix := table.TableNameCamel()
	name := fmt.Sprintf("%sGetAllBy%sList", prefix, primaryKeyCamel)
	tpl = fmt.Sprintf("{{define \"%s\"}}\nselect * from `%s`  where `%s` in ({{in . .%sList}})", name, table.TableName, table.PrimaryKey, primaryKeyCamel)
	if table.DeleteColumn != "" {
		tpl = fmt.Sprintf("%s  and `%s` is null", tpl, table.DeleteColumn)
	}
	tpl = tpl + ";\n{{end}}\n"
	return
}

func tplGetByPrimaryKey(table *ddlparser.Table) (tpl string, err error) {
	primaryKeyCamel := table.PrimaryKeyCamel()
	prefix := table.TableNameCamel()

	name := fmt.Sprintf("%sGetBy%s", prefix, primaryKeyCamel)
	tpl = fmt.Sprintf("{{define \"%s\"}}\nselect * from `%s`  where `%s`=:%s", name, table.TableName, table.PrimaryKey, primaryKeyCamel)
	if table.DeleteColumn != "" {
		tpl = fmt.Sprintf("%s  and `%s` is null", tpl, table.DeleteColumn)
	}
	tpl = tpl + ";\n{{end}}\n"
	return
}

func tplPaginateWhereName(tableNameCamel string) string {
	prefix := tableNameCamel

	return fmt.Sprintf("%sPaginateWhere", prefix)
}

func tplPaginateWhere(table *ddlparser.Table) (tpl string, err error) {
	tpl = fmt.Sprintf("{{define \"%s\"}}\n  ", tplPaginateWhereName(table.TableNameCamel()))

	tpl = tpl + "\n{{end}}\n"
	return
}

func tplPaginateTotal(table *ddlparser.Table) (tpl string, err error) {
	prefix := table.TableNameCamel()
	name := fmt.Sprintf("%sPaginateTotal", prefix)
	tpl = fmt.Sprintf("{{define \"%s\"}}\nselect count(*) as `count` from `%s`  where 1=1 {{template \"%s\" .}} ", name, table.TableName, tplPaginateWhereName(table.TableNameCamel()))
	if table.DeleteColumn != "" {
		tpl = fmt.Sprintf("%s  and `%s` is null", tpl, table.DeleteColumn)
	}
	tpl = tpl + ";\n{{end}}\n"
	return
}

func tplPaginate(table *ddlparser.Table) (tpl string, err error) {

	prefix := table.TableNameCamel()
	name := fmt.Sprintf("%sPaginate", prefix)
	tpl = fmt.Sprintf("{{define \"%s\"}}\nselect * from `%s`  where 1=1 {{template \"%s\" .}} ", name, table.TableName, tplPaginateWhereName(table.TableNameCamel()))
	if table.DeleteColumn != "" {
		tpl = fmt.Sprintf("%s  and `%s` is null", tpl, table.DeleteColumn)
	}
	updatedAtColumn := table.UpdatedAtColumn()
	if updatedAtColumn != nil {
		tpl = fmt.Sprintf(" %s order by `%s` desc ", tpl, updatedAtColumn.Name)
	}
	tpl = fmt.Sprintf(" %s limit :Offset,:Limit ", tpl)
	tpl = tpl + ";\n{{end}}\n"
	return
}

func tplInsert(table *ddlparser.Table) (tpl string, err error) {
	prefix := table.TableNameCamel()
	name := fmt.Sprintf("%sInsert", prefix)
	columns := make([]string, 0)
	values := make([]string, 0)
	for _, column := range table.Columns {
		if isIgnoreColumn(column, table) {
			continue
		}
		columns = append(columns, Backquote(column.ColumnName))
		values = append(values, ":"+column.CamelName)
	}

	columnStr := strings.Join(columns, ",")
	valueStr := strings.Join(values, ",")
	tpl = fmt.Sprintf("{{define \"%s\"}}\ninsert into `%s` (%s)values\n (%s);\n{{end}}\n", name, table.TableName, columnStr, valueStr)

	return
}

func tplUpdate(table *ddlparser.Table) (tpl string, err error) {
	prefix := table.TableNameCamel()
	name := fmt.Sprintf("%sUpdate", prefix)
	updataList := make([]string, 0)
	for _, column := range table.Columns {
		if isIgnoreColumn(column, table) {
			continue
		}
		str := fmt.Sprintf("{{if .%s}} {{$preComma.PreComma}} `%s`=:%s {{end}} ", column.CamelName, column.ColumnName, column.CamelName)
		updataList = append(updataList, str)
	}
	updateTpl := strings.Join(updataList, "\n")
	tpl = fmt.Sprintf("{{define \"%s\"}}\n{{$preComma:=newPreComma}}\n update `%s` set %s where `%s`=:%s;\n{{end}}\n", name, table.TableName, updateTpl, table.PrimaryKey, table.PrimaryKeyCamel())
	return
}

func tplDel(table *ddlparser.Table) (tpl string, err error) {
	prefix := table.TableNameCamel()
	name := fmt.Sprintf("%sDel", prefix)
	tpl = fmt.Sprintf("{{define \"%s\"}}\nupdate `%s` set `%s`={{currentTime .}} where `%s`=:%s;\n{{end}}\n", name, table.TableName, table.DeleteColumn, table.PrimaryKey, table.PrimaryKeyCamel())
	return
}

func isIgnoreColumn(column *ddlparser.Column, table *ddlparser.Table) (yes bool) {
	if column.AutoIncrement { // 自增列,忽略
		return true
	}
	columnName := column.Name
	if column.IsDefaultValueCurrentTimestamp() { // 自动填充时间的列,忽略
		return true
	}

	ignoreColumnMap := make(map[string]string)
	ignoreColumnMap[table.DeleteColumn] = table.DeleteColumn
	_, yes = ignoreColumnMap[columnName]
	return
}

func Backquote(s string) (out string) {
	out = fmt.Sprintf("`%s`", s)
	return
}
