package http

import (
	"math"
	"strconv"
	"time"

	"github.com/mipecx/rrt_system/backend/internal/ws"
)

// StartRRTSimulator — симулятор: двигает экипаж RRT по кругу вокруг Walking Street (Паттайя)
//
// ВАЖНО: статус здесь захардкожен как "en_route" и никак не связан с реальным
// статусом экипажа в БД. Если этот же экипаж (55555555-...) будет назначен на
// инцидент/освобождён через AssignCrew, симулятор всё равно продолжит слать
// "en_route" поверх — визуально экипаж снова "оживёт" как занятый. Стоит либо
// убрать симулятор перед продакшеном, либо читать актуальный статус из БД.
func StartRRTSimulator(hub *ws.Hub) {
	go func() {
		centerLat := 12.9255
		centerLng := 100.8729
		radius := 0.003 // Радиус круга в градусах (~300 метров)
		angle := 0.0

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			angle += 0.1 // Скорость вращения
			if angle > 2*math.Pi {
				angle = 0
			}

			// Вычисляем новые координаты по окружности
			newLat := centerLat + radius*math.Cos(angle)
			newLng := centerLng + radius*math.Sin(angle)

			// Формируем JSON-пакет обновления для фронтенда
			jsonMsg := []byte(
				`{"type": "RRT_UPDATE", "data": {` +
					`"id": "55555555-1111-1111-1111-111111111111",` +
					`"name": "RRT Crew Alpha (Bike-01)",` +
					`"status": "en_route",` +
					`"lat": ` + dtos(newLat) + `,` +
					`"lng": ` + dtos(newLng) + `}}`,
			)

			hub.Broadcast(jsonMsg)
		}
	}()
}

func dtos(val float64) string {
	return strconv.FormatFloat(val, 'f', 6, 64)
}
