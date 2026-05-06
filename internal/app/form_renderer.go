package app

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"

	"f2m-golang/internal/charset"
	"f2m-golang/internal/config"
)

/**
 * 固定HTMLフォーム描画。
 *
 * 入力フォームHTMLをDOMとして読み込み、入力値とエラーを反映する処理。
 */
func renderFixedFormFile(templatePath string, formConfig config.FormConfig, fieldValues FieldValues, formErrors FormErrors) (string, error) {
	templateHTML, _, err := charset.ReadFile(templatePath)
	if err != nil {
		return "", err
	}

	rootNode, err := html.Parse(strings.NewReader(templateHTML))
	if err != nil {
		return "", err
	}

	formNode := findFormNode(rootNode, formConfig.ID)
	if formNode == nil {
		return "", fmt.Errorf("F2M_ID に対応する form がありません: %s", formConfig.ID)
	}

	// ---------------------------------------------
	// 入力値・エラー反映
	// ---------------------------------------------
	insertHoneypotField(formNode, formConfig)
	prepareAttachmentForm(formNode, formConfig)
	restoreFormValues(formNode, fieldValues)

	if formErrors.HasErrors() {
		insertErrorSummary(formNode, formErrors)
		insertFieldErrors(formNode, formErrors)
	}

	var renderedHTML strings.Builder
	if err := html.Render(&renderedHTML, rootNode); err != nil {
		return "", err
	}

	return renderedHTML.String(), nil
}

/**
 * 対象フォーム検索。
 *
 * hiddenのF2M_IDが一致するform要素を取得する処理。
 */
func findFormNode(rootNode *html.Node, formID string) *html.Node {
	var targetFormNode *html.Node

	walkHTML(rootNode, func(node *html.Node) bool {
		if targetFormNode != nil {
			return false
		}

		if !isElementNode(node, "form") {
			return true
		}

		if formHasID(node, formID) {
			targetFormNode = node
			return false
		}

		return true
	})

	return targetFormNode
}

/**
 * フォームID判定。
 *
 * form配下のhidden F2M_IDが指定値と一致するかを返す処理。
 */
func formHasID(formNode *html.Node, formID string) bool {
	matched := false

	walkHTML(formNode, func(node *html.Node) bool {
		if matched {
			return false
		}

		if !isElementNode(node, "input") {
			return true
		}

		name, _ := getAttribute(node, "name")
		inputType, _ := getAttribute(node, "type")
		value, _ := getAttribute(node, "value")

		if name == "F2M_ID" && strings.EqualFold(inputType, "hidden") && value == formID {
			matched = true
			return false
		}

		return true
	})

	return matched
}

/**
 * 入力値復元。
 *
 * form配下の入力要素へPOST済み値を反映する処理。
 */
func restoreFormValues(formNode *html.Node, fieldValues FieldValues) {
	if len(fieldValues) == 0 {
		return
	}

	walkHTML(formNode, func(node *html.Node) bool {
		switch {
		case isElementNode(node, "input"):
			restoreInputValue(node, fieldValues)
		case isElementNode(node, "textarea"):
			restoreTextareaValue(node, fieldValues)
		case isElementNode(node, "select"):
			restoreSelectValue(node, fieldValues)
		}

		return true
	})
}

/**
 * honeypot項目挿入。
 *
 * bot検知用の非表示入力項目を対象フォーム内へ追加する処理。
 */
func insertHoneypotField(formNode *html.Node, formConfig config.FormConfig) {
	honeypotField := strings.TrimSpace(formConfig.HoneypotField)
	if !formConfig.HoneypotEnabled || honeypotField == "" {
		return
	}

	honeypotNode := newElementNode(
		"div",
		html.Attribute{Key: "class", Val: "f2m-honeypot"},
		html.Attribute{Key: "aria-hidden", Val: "true"},
		html.Attribute{Key: "style", Val: "position:absolute;left:-10000px;top:auto;width:1px;height:1px;overflow:hidden;"},
	)
	honeypotNode.AppendChild(newElementNode("label", html.Attribute{Key: "for", Val: honeypotField}))
	honeypotNode.LastChild.AppendChild(newTextNode("Webサイト"))
	honeypotNode.AppendChild(newElementNode(
		"input",
		html.Attribute{Key: "id", Val: honeypotField},
		html.Attribute{Key: "type", Val: "text"},
		html.Attribute{Key: "name", Val: honeypotField},
		html.Attribute{Key: "value", Val: ""},
		html.Attribute{Key: "tabindex", Val: "-1"},
		html.Attribute{Key: "autocomplete", Val: "off"},
	))

	if formNode.FirstChild == nil {
		formNode.AppendChild(honeypotNode)
		return
	}

	formNode.InsertBefore(honeypotNode, formNode.FirstChild)
}

/**
 * 添付フォーム属性補完。
 *
 * 添付項目が設定されているフォームへmultipart属性を追加する処理。
 */
func prepareAttachmentForm(formNode *html.Node, formConfig config.FormConfig) {
	if len(formConfig.AttachFields) == 0 {
		return
	}

	setAttribute(formNode, "enctype", "multipart/form-data")
}

/**
 * input値復元。
 *
 * input要素の種別に応じてvalueまたはcheckedを反映する処理。
 */
func restoreInputValue(inputNode *html.Node, fieldValues FieldValues) {
	fieldName, ok := getAttribute(inputNode, "name")
	if !ok {
		return
	}

	_, exists := fieldValues[fieldName]
	if !exists {
		return
	}

	inputType, _ := getAttribute(inputNode, "type")
	switch strings.ToLower(inputType) {
	case "button", "file", "image", "reset", "submit":
		return
	case "checkbox", "radio":
		optionValue, ok := getAttribute(inputNode, "value")
		if !ok {
			optionValue = "on"
		}

		if fieldValues.Contains(fieldName, optionValue) {
			setAttribute(inputNode, "checked", "checked")
		} else {
			removeAttribute(inputNode, "checked")
		}
	default:
		setAttribute(inputNode, "value", fieldValues.First(fieldName))
	}
}

/**
 * textarea値復元。
 *
 * textarea要素の子ノードを入力値で置き換える処理。
 */
func restoreTextareaValue(textareaNode *html.Node, fieldValues FieldValues) {
	fieldName, ok := getAttribute(textareaNode, "name")
	if !ok {
		return
	}

	_, exists := fieldValues[fieldName]
	if !exists {
		return
	}

	removeChildren(textareaNode)
	textareaNode.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: fieldValues.First(fieldName),
	})
}

/**
 * select値復元。
 *
 * select配下のoption要素へselected状態を反映する処理。
 */
func restoreSelectValue(selectNode *html.Node, fieldValues FieldValues) {
	fieldName, ok := getAttribute(selectNode, "name")
	if !ok {
		return
	}

	_, exists := fieldValues[fieldName]
	if !exists {
		return
	}

	walkHTML(selectNode, func(node *html.Node) bool {
		if !isElementNode(node, "option") {
			return true
		}

		optionValue, ok := getAttribute(node, "value")
		if !ok {
			optionValue = strings.TrimSpace(textContent(node))
		}

		if fieldValues.Contains(fieldName, optionValue) {
			setAttribute(node, "selected", "selected")
		} else {
			removeAttribute(node, "selected")
		}

		return true
	})
}

/**
 * エラー概要挿入。
 *
 * form先頭へ入力エラー一覧を追加する処理。
 */
func insertErrorSummary(formNode *html.Node, formErrors FormErrors) {
	summaryNode := newElementNode("div", html.Attribute{Key: "class", Val: "f2m-error-summary"}, html.Attribute{Key: "role", Val: "alert"})
	listNode := newElementNode("ul")
	summaryNode.AppendChild(listNode)

	for _, message := range formErrors.Summary {
		itemNode := newElementNode("li")
		itemNode.AppendChild(newTextNode(message))
		listNode.AppendChild(itemNode)
	}

	if formNode.FirstChild == nil {
		formNode.AppendChild(summaryNode)
		return
	}

	formNode.InsertBefore(summaryNode, formNode.FirstChild)
}

/**
 * 項目別エラー挿入。
 *
 * 入力要素の直後へ項目別エラーを追加する処理。
 */
func insertFieldErrors(formNode *html.Node, formErrors FormErrors) {
	lastFieldNodes := make(map[string]*html.Node)

	walkHTML(formNode, func(node *html.Node) bool {
		fieldName, ok := formFieldName(node)
		if !ok {
			return true
		}

		messages := formErrors.Fields[fieldName]
		if len(messages) == 0 {
			return true
		}

		errorID := "f2m-error-" + fieldName
		setAttribute(node, "aria-invalid", "true")
		setAttribute(node, "aria-describedby", errorID)
		lastFieldNodes[fieldName] = node

		return true
	})

	walkHTML(formNode, func(node *html.Node) bool {
		fieldName, ok := formFieldName(node)
		if !ok || lastFieldNodes[fieldName] != node {
			return true
		}

		messages := formErrors.Fields[fieldName]
		if len(messages) == 0 {
			return true
		}

		errorNode := newElementNode(
			"p",
			html.Attribute{Key: "class", Val: "f2m-field-error"},
			html.Attribute{Key: "id", Val: "f2m-error-" + fieldName},
			html.Attribute{Key: "data-f2m-error-for", Val: fieldName},
		)
		errorNode.AppendChild(newTextNode(strings.Join(messages, " ")))

		insertAfter(node, errorNode)

		return true
	})
}

/**
 * フォーム項目名取得。
 *
 * input、textarea、selectのname属性を取得する処理。
 */
func formFieldName(node *html.Node) (string, bool) {
	if !isElementNode(node, "input") && !isElementNode(node, "textarea") && !isElementNode(node, "select") {
		return "", false
	}

	return getAttribute(node, "name")
}

/**
 * HTML走査。
 *
 * falseが返るまで深さ優先でノードを巡回する処理。
 */
func walkHTML(node *html.Node, visitor func(*html.Node) bool) bool {
	if node == nil {
		return true
	}

	if !visitor(node) {
		return false
	}

	for childNode := node.FirstChild; childNode != nil; childNode = childNode.NextSibling {
		if !walkHTML(childNode, visitor) {
			return false
		}
	}

	return true
}

/**
 * 要素ノード判定。
 *
 * 指定タグ名のHTML要素かを返す処理。
 */
func isElementNode(node *html.Node, tagName string) bool {
	return node != nil && node.Type == html.ElementNode && node.Data == tagName
}

/**
 * 属性取得。
 *
 * 指定された属性値と存在有無を返す処理。
 */
func getAttribute(node *html.Node, key string) (string, bool) {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val, true
		}
	}

	return "", false
}

/**
 * 属性設定。
 *
 * 既存属性の更新または新規属性の追加を行う処理。
 */
func setAttribute(node *html.Node, key string, value string) {
	for index, attribute := range node.Attr {
		if attribute.Key == key {
			node.Attr[index].Val = value
			return
		}
	}

	node.Attr = append(node.Attr, html.Attribute{Key: key, Val: value})
}

/**
 * 属性削除。
 *
 * 指定属性を要素から取り除く処理。
 */
func removeAttribute(node *html.Node, key string) {
	attributes := node.Attr[:0]
	for _, attribute := range node.Attr {
		if attribute.Key != key {
			attributes = append(attributes, attribute)
		}
	}

	node.Attr = attributes
}

/**
 * 子ノード削除。
 *
 * 指定ノード配下の子ノードをすべて取り除く処理。
 */
func removeChildren(node *html.Node) {
	for node.FirstChild != nil {
		node.RemoveChild(node.FirstChild)
	}
}

/**
 * 兄弟ノード挿入。
 *
 * 指定ノードの直後へ新規ノードを追加する処理。
 */
func insertAfter(referenceNode *html.Node, newNode *html.Node) {
	parentNode := referenceNode.Parent
	if parentNode == nil {
		return
	}

	if referenceNode.NextSibling == nil {
		parentNode.AppendChild(newNode)
		return
	}

	parentNode.InsertBefore(newNode, referenceNode.NextSibling)
}

/**
 * 要素ノード生成。
 *
 * 指定タグ名と属性を持つHTML要素を生成する処理。
 */
func newElementNode(tagName string, attributes ...html.Attribute) *html.Node {
	return &html.Node{
		Type: html.ElementNode,
		Data: tagName,
		Attr: attributes,
	}
}

/**
 * テキストノード生成。
 *
 * HTML描画時にエスケープされるテキストノードを生成する処理。
 */
func newTextNode(text string) *html.Node {
	return &html.Node{
		Type: html.TextNode,
		Data: text,
	}
}

/**
 * テキスト内容取得。
 *
 * 配下のテキストノードを連結して返す処理。
 */
func textContent(node *html.Node) string {
	var content strings.Builder

	walkHTML(node, func(currentNode *html.Node) bool {
		if currentNode.Type == html.TextNode {
			content.WriteString(currentNode.Data)
		}

		return true
	})

	return content.String()
}
