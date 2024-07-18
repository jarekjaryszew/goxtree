package goxtree

import (
	"fmt"
	"reflect"
	"syscall/js"
)

var supportedAttrs = []string{"class", "id", "href", "style"}

type domNode struct {
	Tag             string
	Text            string
	Id              string
	Children        []*domNode
	ForeignChildren []*CoreNode
	Atrrs           map[string]string
	Owner           *CoreNode
}

type CoreNode struct {
	domNode
	hostId              string
	ElementsWithId      map[string]*domNode
	EventListeners      map[EventListenerKey]EventListenerVal
	RegisteredListeners map[*js.Func]bool
	ForeignChildren     map[string]*CoreNode
	Postfix             string
}

type EventListenerKey struct {
	Id    string
	Event string
}

type EventListenerVal struct {
	Callback func()
	JsFunc   *js.Func
}

func eventListenerWrapper(cb func()) *js.Func {
	jsFunc := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		cb()
		return nil
	})
	return &jsFunc
}

func (cn *CoreNode) AddEventListenerToElementWithId(id string, event string, cb func()) {

	// fmt.Println("adding event listener", event, "to element with id", id)
	cn.EventListeners[EventListenerKey{Id: id, Event: event}] = EventListenerVal{Callback: cb, JsFunc: eventListenerWrapper(cb)}
}

func (cn *CoreNode) registerEventListeners() {
	for id, listener := range cn.EventListeners {
		js.Global().Get("document").Call("getElementById", id.Id).Call("addEventListener", id.Event, *listener.JsFunc)
		cn.RegisteredListeners[listener.JsFunc] = true
	}
}

func (cn *CoreNode) deregisterEventListeners() {
	for listener, _ := range cn.RegisteredListeners {
		listener.Release()
	}
	cn.RegisteredListeners = make(map[*js.Func]bool)
}

func (cn *CoreNode) ReadValueFromElementWithId(id string) string {
	return js.Global().Get("document").Call("getElementById", id).Get("value").String()
}

func (cn *CoreNode) AddChildToElementWithId(id string, dn *CoreNode) {
	cn.ElementsWithId[id].ForeignChildren = append(cn.ElementsWithId[id].ForeignChildren, dn)
	cn.ForeignChildren[dn.Id] = dn
	dn.hostId = id
}

func (cn *CoreNode) ClearChildrenFromElementWithId(id string) {
	for _, c := range cn.ElementsWithId[id].ForeignChildren {
		c.deregisterEventListeners()
		delete(cn.ForeignChildren, c.Id)
	}
	cn.ElementsWithId[id].ForeignChildren = []*CoreNode{}
}

func (cn *CoreNode) RemoveChildFromElementWithId(id string, childId string) {
	for i, c := range cn.ElementsWithId[id].ForeignChildren {
		if c.Id == childId {
			c.deregisterEventListeners()
			cn.ElementsWithId[id].ForeignChildren = append(cn.ElementsWithId[id].ForeignChildren[:i], cn.ElementsWithId[id].ForeignChildren[i+1:]...)
			delete(cn.ForeignChildren, childId)
			return
		}
	}
}

func (cn *CoreNode) SetAttributeToElementWithId(id string, attr string, val string) {
	cn.ElementsWithId[id].Atrrs[attr] = val
}

func (cn *CoreNode) GetAttributeFromElementWithId(id string, attr string) string {
	return cn.ElementsWithId[id].Atrrs[attr]
}

func (cn *CoreNode) SetTextToElementWithId(id string, text string) {
	cn.ElementsWithId[id].Text = text
}

func (cn *CoreNode) GetTextFromElementWithId(id string) string {
	return cn.ElementsWithId[id].Text
}

func (cn *CoreNode) MountToNode(id string) {
	cn.hostId = id
	cn.Render()
}

func (cn *CoreNode) Render() {
	js.Global().Get("document").Call("getElementById", cn.hostId).Set("innerHTML", cn.ToHtml())
	cn.deregisterEventListeners()
	cn.registerEventListeners()
	for _, c := range cn.ForeignChildren {
		c.registerEventListeners()
	}
}

func (cn *CoreNode) RenderFromElementWithId(id string) {
	el := cn.ElementsWithId[id]
	js.Global().Get("document").Call("getElementById", id).Set("outerHTML", el.ToHtml())
	for _, c := range cn.ForeignChildren {
		c.registerEventListeners()
	}
}

func extractAttributes(descriptor reflect.StructField, postfix string) map[string]string {
	attributes := make(map[string]string)
	for _, v := range supportedAttrs {
		if attr, ok := descriptor.Tag.Lookup(v); ok {
			if v == "id" {
				attr = attr + postfix
			}
			attributes[v] = attr
		}
	}
	return attributes
}

// Creates a CoreNode from a template struct
// Second argument is a postifx for the id of all the nodes.
// It is useful when you want to create multiple instances of the same template like list items
func DressDomTree[T any](descriptor *T, idPostfix string) (*CoreNode, error) {
	cn := &CoreNode{}
	descriptorType := reflect.TypeOf(*descriptor)
	me, ok := descriptorType.FieldByName("me")
	if !ok {
		fmt.Println("no 'me' field in the root of the struct")
		return nil, fmt.Errorf("no 'me' field in the root of the struct")
	}
	cn.Postfix = idPostfix
	cn.Id = me.Tag.Get("id") + idPostfix
	cn.Tag = me.Tag.Get("tag")
	cn.Text = me.Tag.Get("text")
	cn.Atrrs = extractAttributes(me, cn.Postfix)
	cn.EventListeners = make(map[EventListenerKey]EventListenerVal)
	cn.ElementsWithId = make(map[string]*domNode)
	cn.RegisteredListeners = make(map[*js.Func]bool)
	cn.ForeignChildren = make(map[string]*CoreNode)
	cn.ElementsWithId[cn.Id] = &cn.domNode
	for i := 0; i < descriptorType.NumField(); i++ {
		field := descriptorType.Field(i)
		if field.Name == "me" {
			continue
		}
		dn := &domNode{
			Owner: cn,
		}
		dn.dressNode(field)
		cn.Children = append(cn.Children, dn)
	}
	return cn, nil
}

func (dn *domNode) dressNode(field reflect.StructField) error {
	dn.Id = field.Tag.Get("id")
	if dn.Id != "" {
		dn.Owner.ElementsWithId[dn.Id] = dn
	}
	dn.Tag = field.Tag.Get("tag")
	dn.Text = field.Tag.Get("text")
	dn.Atrrs = extractAttributes(field, dn.Owner.Postfix)

	if field.Type.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < field.Type.NumField(); i++ {
		childField := field.Type.Field(i)
		// fmt.Println("field", cf.Name, "type", cf.Type)
		childNode := &domNode{
			Owner: dn.Owner,
		}
		childNode.dressNode(childField)
		dn.Children = append(dn.Children, childNode)
	}
	return nil
}

func (dn *domNode) ToHtml() string {
	html := "<" + dn.Tag
	for _, a := range supportedAttrs {
		attr, ok := dn.Atrrs[a]
		if ok {
			html = html + " " + a + "=\"" + attr + "\""
		}
	}
	html = html + ">" + dn.Text
	for _, c := range dn.Children {
		html = html + c.ToHtml()
	}
	for _, c := range dn.ForeignChildren {
		html = html + c.ToHtml()
	}
	html = html + "</" + dn.Tag + ">"
	return html
}
