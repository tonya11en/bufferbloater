// Code generated by the FlatBuffers compiler. DO NOT EDIT.

//nolint
package bouncer_flatbuf

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type PushRecordReply struct {
	_tab flatbuffers.Table
}

func GetRootAsPushRecordReply(buf []byte, offset flatbuffers.UOffsetT) *PushRecordReply {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &PushRecordReply{}
	x.Init(buf, n+offset)
	return x
}

func (rcv *PushRecordReply) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *PushRecordReply) Table() flatbuffers.Table {
	return rcv._tab
}

func PushRecordReplyStart(builder *flatbuffers.Builder) {
	builder.StartObject(0)
}
func PushRecordReplyEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
