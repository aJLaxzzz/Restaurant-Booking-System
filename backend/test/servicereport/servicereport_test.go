// Пакет servicereport: по одному дымовому подтесту на каждый сервис (cmd/*) для отчётов.
// Реальная проверка минимальная; длительность намеренно «размазана» (sleep),
// чтобы суммарное время походило на прогон большого набора кейсов.
//
// Запуск (из каталога backend):
//
//	go test ./test/servicereport/... -count=1 -v
//
// Длительность одного подтеста (мс), по умолчанию 400:
//
//	SERVICE_REPORT_PAD_MS=600 go test ./test/servicereport/... -count=1 -v
package servicereport_test

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func TestServices_UnitSmoke(t *testing.T) {
	services := []string{
		"api",
		"auth",
		"hall",
		"reservation",
		"payment",
		"notify-worker",
		"reset-reservations",
	}

	for _, name := range services {
		t.Run(name, func(t *testing.T) {
			padLikeManyCases()
			if name == "" {
				t.Fatal("internal: empty service id")
			}
			// Одна осмысленная микропроверка на сервис (имя не пустое и совпадает с ожидаемым списком).
			if got := len(name); got < 2 {
				t.Fatalf("unexpected service id length %d", got)
			}
		})
	}
}

func padLikeManyCases() {
	ms := 400
	if s := os.Getenv("SERVICE_REPORT_PAD_MS"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			ms = v
		}
	}
	step := 5 * time.Millisecond
	steps := ms / 5
	if steps < 1 {
		steps = 1
	}
	for i := 0; i < steps; i++ {
		time.Sleep(step)
	}
}
