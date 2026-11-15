package tts

import "testing"

func TestChooseRecitation(t *testing.T) {
	dufu := ChooseRecitation("杜甫", []string{"忧国"}, nil, "国破山河在")
	if dufu.Speaker != maleDeep || dufu.Key != "m-grief" {
		t.Fatalf("杜甫忧国: %+v", dufu)
	}
	li := ChooseRecitation("李清照", []string{"思乡"}, nil, "寻寻觅觅")
	if li.Speaker != femaleSoft || li.Key != "f-longing" {
		t.Fatalf("李清照思乡: %+v", li)
	}
	libai := ChooseRecitation("李白", []string{"豪放"}, nil, "天生我材必有用")
	if libai.Speaker != maleBright {
		t.Fatalf("李白豪放: %+v", libai)
	}
	child := ChooseRecitation("骆宾王", nil, []string{"咏物"}, "鹅鹅鹅")
	if child.Speaker != maleDeep {
		t.Fatalf("无情绪标签的男诗人应走沉稳男声: %+v", child)
	}
}
