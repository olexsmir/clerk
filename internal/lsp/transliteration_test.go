package lsp

import "testing"

func TestLatinToCyrillic(t *testing.T) {
	tests := map[string]string{
		"":         "",
		"vydatky":  "видатки",
		"gotivka":  "готівка",
		"kava":     "кава",
		"vy":       "ви",
		"tramvaj":  "трамвай",
		"yizhak":   "їжак",
		"jidlo":    "їдло",
		"shchuka":  "щука",
		"schuka":   "щука",
		"khlib":    "хліб",
		"hata":     "хата",
		"groshi":   "гроші",
		"yevgen":   "євген",
		"cukor":    "цукор",
		"dzerkalo": "дзеркало",
		"dzhaz":    "джаз",
		"oleksij":  "олексій",
		"KAVA":     "кава",
		"кофe":     "кофе",
		"кофе":     "кофе",
	}
	for in, out := range tests {
		t.Run(in, func(t *testing.T) {
			if got := latinToCyrillic(in); got != out {
				t.Errorf("latinToCyrillic(%q) = %q, want %q", in, got, out)
			}
		})
	}
}

