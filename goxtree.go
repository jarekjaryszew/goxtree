package goxtree

import (
	"fmt"
	"reflect"
	"syscall/js"
)

var supportedAttrs = []string{"class", "id", "href", "style"}

type CallBackKey struct {
	Id    string
	Event string
}

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
	EventListeners      map[CallBackKey]*js.Func
	RegisteredListeners map[*js.Func]bool
	ForeignChildren     map[*CoreNode]bool
}

func (cn *CoreNode) AddChildToElementWithId(id string, dn *CoreNode) {
	cn.ElementsWithId[id].ForeignChildren = append(cn.ElementsWithId[id].ForeignChildren, dn)
	cn.ForeignChildren[dn] = true
	dn.hostId = id
}

func (cn *CoreNode) ClearChildrenFromElementWithId(id string) {
	cn.ElementsWithId[id].ForeignChildren = []*CoreNode{}
	cn.ForeignChildren = make(map[*CoreNode]bool)
}

func (cn *CoreNode) AddEventListenerToElementWithId(id string, event string, cb *js.Func) {
	// fmt.Println("adding event listener", event, "to element with id", id)
	cn.EventListeners[CallBackKey{Id: id, Event: event}] = cb
}

func (cn *CoreNode) SetTextToElementWithId(id string, text string) {
	cn.ElementsWithId[id].Text = text
}

func (cn *CoreNode) registerEventListeners() {
	for id, listener := range cn.EventListeners {
		// fmt.Println("registering event listener", id.Event, "to element with id", id)
		js.Global().Get("document").Call("getElementById", id.Id).Call("addEventListener", id.Event, *listener)
		cn.RegisteredListeners[listener] = true
	}
}

func (cn *CoreNode) MountToTag(id string) {
	cn.hostId = id
	cn.Render()
}

func (cn *CoreNode) Render() {
	js.Global().Get("document").Call("getElementById", cn.hostId).Set("innerHTML", cn.ToHtml())
	cn.registerEventListeners()
	for c, _ := range cn.ForeignChildren {
		c.registerEventListeners()
	}
}

func extractAttributes(descriptor reflect.StructField) map[string]string {
	attributes := make(map[string]string)
	for _, v := range supportedAttrs {
		if attr, ok := descriptor.Tag.Lookup(v); ok {
			attributes[v] = attr
		}
	}
	return attributes
}

func DressDomTree[T any](descriptor *T) (*CoreNode, error) {
	cn := &CoreNode{}
	descriptorType := reflect.TypeOf(*descriptor)
	me, ok := descriptorType.FieldByName("me")
	if !ok {
		fmt.Println("no 'me' field in the root of the struct")
		return nil, fmt.Errorf("no 'me' field in the root of the struct")
	}
	cn.Id = me.Tag.Get("id")
	cn.Tag = me.Tag.Get("tag")
	cn.Text = me.Tag.Get("text")
	cn.Atrrs = extractAttributes(me)
	cn.EventListeners = make(map[CallBackKey]*js.Func)
	cn.ElementsWithId = make(map[string]*domNode)
	cn.RegisteredListeners = make(map[*js.Func]bool)
	cn.ForeignChildren = make(map[*CoreNode]bool)
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
	dn.Atrrs = extractAttributes(field)

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
