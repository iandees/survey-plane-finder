package geojson

import "math"

func SimplifyTrack(coords []Coordinate, epsilon float64) []Coordinate {
	if len(coords) <= 2 {
		return coords
	}
	return douglasPeucker(coords, epsilon)
}

func douglasPeucker(coords []Coordinate, epsilon float64) []Coordinate {
	if len(coords) <= 2 {
		return coords
	}

	maxDist := 0.0
	maxIdx := 0
	first := coords[0]
	last := coords[len(coords)-1]

	for i := 1; i < len(coords)-1; i++ {
		d := perpendicularDistance(coords[i], first, last)
		if d > maxDist {
			maxDist = d
			maxIdx = i
		}
	}

	if maxDist > epsilon {
		left := douglasPeucker(coords[:maxIdx+1], epsilon)
		right := douglasPeucker(coords[maxIdx:], epsilon)
		return append(left[:len(left)-1], right...)
	}

	return []Coordinate{first, last}
}

func perpendicularDistance(point, lineStart, lineEnd Coordinate) float64 {
	dx := lineEnd[0] - lineStart[0]
	dy := lineEnd[1] - lineStart[1]

	if dx == 0 && dy == 0 {
		return math.Sqrt(math.Pow(point[0]-lineStart[0], 2) + math.Pow(point[1]-lineStart[1], 2))
	}

	t := ((point[0]-lineStart[0])*dx + (point[1]-lineStart[1])*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))

	projX := lineStart[0] + t*dx
	projY := lineStart[1] + t*dy

	return math.Sqrt(math.Pow(point[0]-projX, 2) + math.Pow(point[1]-projY, 2))
}
