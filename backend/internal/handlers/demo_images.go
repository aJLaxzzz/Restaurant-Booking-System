package handlers

import (
	"encoding/json"
)

// Канонические slug демо-ресторанов (совпадают с колонкой restaurants.slug).
const (
	DemoSlugTrattoria = "trattoria-tverskaya"
	DemoSlugLaLuna    = "la-luna"
	DemoSlugSakura    = "sakura-lite"
	DemoSlugBella     = "bella-vista"
)

// demoRestaurantCoverURL — обложка для списка и hero (первое фото набора).
func demoRestaurantCoverURL(slug string) string {
	g := demoRestaurantGalleryURLs(slug)
	if len(g) == 0 {
		return ""
	}
	return g[0]
}

// demoRestaurantGalleryURLs — все фото ресторана для карточки (порядок = порядок показа).
func demoRestaurantGalleryURLs(slug string) []string {
	switch slug {
	case DemoSlugTrattoria:
		return []string{
			"/demo/restaurants/tratoria1.webp",
			"/demo/restaurants/tratoria2.webp",
			"/demo/restaurants/tratoria3.webp",
		}
	case DemoSlugLaLuna:
		return []string{
			"/demo/restaurants/laluna1.webp",
			"/demo/restaurants/laluna2.jpg",
			"/demo/restaurants/laluna3.jpg",
		}
	case DemoSlugSakura:
		return []string{
			"/demo/restaurants/japanrest1.jpeg",
			"/demo/restaurants/japanrest2.webp",
			"/demo/restaurants/japanrest3.jpg",
		}
	case DemoSlugBella:
		return []string{
			"/demo/restaurants/bella1.webp",
			"/demo/restaurants/bella2.webp",
			"/demo/restaurants/bella3.jpeg",
		}
	default:
		return nil
	}
}

// demoRestaurantExtraJSONForSeed — contact_email + photo_gallery для INSERT в restaurants.extra_json.
func demoRestaurantExtraJSONForSeed(contactEmail, slug string) []byte {
	m := map[string]any{
		"contact_email": contactEmail,
		"photo_gallery": demoRestaurantGalleryURLs(slug),
	}
	b, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// demoMenuImagesForSlug — порядок URL совпадает с порядком позиций в seedMenu* (sort_order, name).
func demoMenuImagesForSlug(slug string) []string {
	switch slug {
	case DemoSlugTrattoria:
		return []string{
			"/demo/dishes/margarita.webp",
			"/demo/dishes/4cheese.webp",
			"/demo/dishes/karbonara.jpg",
			"/demo/dishes/tiramisu.jpg",
			"/demo/dishes/lemonade.webp",
			"/demo/dishes/espresso.jpg",
		}
	case DemoSlugLaLuna:
		return []string{
			"/demo/dishes/duck.jpg",
			"/demo/dishes/risotto.webp",
			"/demo/dishes/tartar.webp",
			"/demo/dishes/cupuchino.jpg",
			"/demo/dishes/lemonade.webp",
		}
	case DemoSlugSakura:
		return []string{
			"/demo/dishes/filadelfia.jpeg",
			"/demo/dishes/california.webp",
			"/demo/dishes/ramen.webp",
			"/demo/dishes/tyahan.jpg",
			"/demo/dishes/miso.jpg",
		}
	case DemoSlugBella:
		return []string{
			"/demo/dishes/bruskette.webp",
			"/demo/dishes/vitello.webp",
			"/demo/dishes/kaprese.jpg",
			"/demo/dishes/karbonara.jpg",
			"/demo/dishes/tailitely.jpg",
			"/demo/dishes/risotto.webp",
			"/demo/dishes/ossobuko.webp",
			"/demo/dishes/fish.jpg",
			"/demo/dishes/pannakota.webp",
			"/demo/dishes/tiramisu.jpg",
			"/demo/dishes/espresso.jpg",
			"/demo/dishes/aperol.jpg",
			"/demo/dishes/mineralwater.webp",
		}
	default:
		return nil
	}
}

// photoGalleryURLsFromExtraMap — значение extra_json.photo_gallery после json.Unmarshal в map[string]any.
func photoGalleryURLsFromExtraMap(m map[string]any) []string {
	if m == nil {
		return nil
	}
	raw, ok := m["photo_gallery"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		var out []string
		for _, x := range v {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
