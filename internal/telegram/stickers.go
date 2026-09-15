package telegram

import (
	"fmt"

	"github.com/malbs/UnoGoBot/internal/uno"
)

var Stickers = map[string]string{
	"option_draw":  "BQADBAAD-AIAAl9XmQABxEjEcFM-VHIC",
	"option_pass":  "BQADBAAD-gIAAl9XmQABcEkAAbaZ4SicAg",
	"option_bluff": "BQADBAADygIAAl9XmQABJoLfB9ntI2UC",
	"option_info":  "BQADBAADxAIAAl9XmQABC5v3Z77VLfEC",
	"colorchooser": "CAADBAADrg4AAvX2mVEpx_BiDIE5nQI",
	"draw_four":    "CAADBAADYRAAArnkmVGmqXHhjWEBxAI",
	"r_0":          "CAADBAAD6A8AAn_ckVHPWHqiBR_3jAI",
	"r_1":          "CAADBAAD5Q0AAg-ImVEx-blQI88RrQI",
	"r_2":          "CAADBAAD1g0AAuMjmVEkQsVhN49DMAI",
	"r_3":          "CAADBAADlhAAAqy4mVHWovoaWfQG_gI",
	"r_4":          "CAADBAADCRoAAqf_kVFnl8ACL1rjpwI",
	"r_5":          "CAADBAADVw8AAjmamVEEv2TVeL9cpQI",
	"r_6":          "CAADBAADHQ4AAuuUkVH2I-yn6nRBVAI",
	"r_7":          "CAADBAADNQ8AArP1kVF5rqHtk0pQ-AI",
	"r_8":          "CAADBAAD1BAAAuQDkVEPiIodUi6WvwI",
	"r_9":          "CAADBAAD2Q4AAq1nkFHM6z5C0Kff2QI",
	"r_draw":       "CAADBAADvQ8AAqZukFGEmkRSoSZQEwI",
	"r_reverse":    "CAADBAAD5RAAAg89mVE8-EY_2DifcAI",
	"r_skip":       "CAADBAADRg4AAp8bmVFOC6xdEZZRwwI",
	"g_0":          "CAADBAADTg4AAoQxmFF07jR_vfB4xgI",
	"g_1":          "CAADBAADQg4AAhkgmFGlsif9nNtXwgI",
	"g_2":          "CAADBAAD2BUAAue_mFGENiPSjZxbiQI",
	"g_3":          "CAADBAADpw4AAjO9mFHAOz8KD2n7BwI",
	"g_4":          "CAADBAADRhAAAqF7kFEcwLalLfDfaAI",
	"g_5":          "CAADBAADAg8AAqXLmFHJyg2F_ybbvwI",
	"g_6":          "CAADBAADVhYAAtK7mVGigRq_EkCuVgI",
	"g_7":          "CAADBAAD2RIAArccmFEj-8LIVNAbsgI",
	"g_8":          "CAADBAAD6AwAAuvmmFHBRarMimOWawI",
	"g_9":          "CAADBAADExEAAsNkmVFr8DaHGOwsggI",
	"g_draw":       "CAADBAADhA8AArxYmVH9ch5Jp00AAboC",
	"g_reverse":    "CAADBAADMhAAAvVOmFGH284LIY7cegI",
	"g_skip":       "CAADBAADbBcAAqinkVEOwkJtDRfk2gI",
	"b_0":          "CAADBAAD-BAAAkj8kFG61GJdw29QOAI",
	"b_1":          "CAADBAADcRMAAu-EmFFT1i4LcqO4OQI",
	"b_2":          "CAADBAAD0xQAAqVhmVHyrFSAbxtfjwI",
	"b_3":          "CAADBAADNg0AAn-xmFHev8IdF_ie0wI",
	"b_4":          "CAADBAADlQ4AAjZamVFcIL_pVB5cFwI",
	"b_5":          "CAADBAADrgwAAuL5mVHvEBZ8CG5p5QI",
	"b_6":          "CAADBAADDhUAAuGRmVGQYvmEOxczBAI",
	"b_7":          "CAADBAADIxEAAv_dmFEuVt39kkgZgwI",
	"b_8":          "CAADBAAD2w0AAoE6kVHG7WscV4F2hwI",
	"b_9":          "CAADBAADvQ0AArRMmVErWaSRP_giKQI",
	"b_draw":       "CAADBAADlw4AAjF_kFHPWSoYKBwtwQI",
	"b_reverse":    "CAADBAADog8AAqDJmVEJQp5WocnUnQI",
	"b_skip":       "CAADBAAD-QwAAgbZmFGltUlnslDNUQI",
	"y_0":          "CAADBAADrQ4AAr5WmVHNf69eBn2YOAI",
	"y_1":          "CAADBAADcg8AAmqKmVHfVeUI3u_i7AI",
	"y_2":          "CAADBAADkA4AAuDImFEQ8qjFlcKplQI",
	"y_3":          "CAADBAAD-QwAAmromFGAqVn-Y8N72wI",
	"y_4":          "CAADBAADjQ4AAmNLmFG80k7kfgx1NAI",
	"y_5":          "CAADBAADqQ8AAmgYmFH1_ey_bMQNYwI",
	"y_6":          "CAADBAADdQ0AAuWcmFEbG_gm1wGYCQI",
	"y_7":          "CAADBAAD6QwAApQAAZhRI8OfRvLX3vkC",
	"y_8":          "CAADBAADARAAAi-2kVEifJ-O9WVilgI",
	"y_9":          "CAADBAADxA0AAhQ8mFHjnl9tUCHSLAI",
	"y_draw":       "CAADBAADzw4AAncZmVEhLhX17eqX8AI",
	"y_reverse":    "CAADBAADTxAAAqgFmVEJRBw4eWgnDwI",
	"y_skip":       "CAADBAADPhYAAiGbkFG9hptFPLgj7wI",
}

var StickersGrey = map[string]string{
	"colorchooser": "CAADBAADpQ4AAlfDmFFHGkwyGFeCFQI",
	"draw_four":    "CAADBAADMRMAAv7amFHvKGLoNyFbNQI",
	"r_0":          "CAADBAADsBMAAuGdkFHTZ-jl4eNn-gI",
	"r_1":          "CAADBAADVA4AAhpfkFEKt19qveGSPgI",
	"r_2":          "CAADBAADrw0AAoWsmVHguULNoYJwUwI",
	"r_3":          "CAADBAADzxMAAjvkkFFdtKJu5WGwUgI",
	"r_4":          "CAADBAAD1Q8AAoHZkFFvyQnFHzfwiQI",
	"r_5":          "CAADBAADWxEAAvkHkFGUo86qxKV0kwI",
	"r_6":          "CAADBAAD_hIAAjx0mVGmlm-b_FHQBAI",
	"r_7":          "CAADBAADmhEAAslomFHOv7bqcDJkDAI",
	"r_8":          "CAADBAADtw0AAgqVmVG2HdSbcJYxZgI",
	"r_9":          "CAADBAADNxEAAuF6mVE3WzTMJkSVAgI",
	"r_draw":       "CAADBAADVxAAAiNukFE1K2xORNnfMwI",
	"r_reverse":    "CAADBAADQxMAAvH0mVHKznpt-uu9ngI",
	"r_skip":       "CAADBAADZA4AApbPkFFB9E2Px-HFpgI",
	"g_0":          "CAADBAAD8w4AAjDEmFG7DwKggUEj9QI",
	"g_1":          "CAADBAAD2g0AAo_DmVHIPG84WdIo1wI",
	"g_2":          "CAADBAADEhEAAoRXmVGIG2nuN45P6AI",
	"g_3":          "CAADBAADug8AAsSRmFFzk0TcRuG8VAI",
	"g_4":          "CAADBAADrQ8AAvgmkFESfo9BjF7-3gI",
	"g_5":          "CAADBAADVhAAAnPqkFFtxtFX9HlT-AI",
	"g_6":          "CAADBAADMg8AAiSBmFHIQw1jFjv6UwI",
	"g_7":          "CAADBAADvREAAv0BkVGDq3H1DCq_DQI",
	"g_8":          "CAADBAADWQ4AAhOEkVG96JDgCtFrEwI",
	"g_9":          "CAADBAAD2xYAAruDmFFAUMFryEwjoAI",
	"g_draw":       "CAADBAADLA4AAu9tkVGTzBbeeYydIQI",
	"g_reverse":    "CAADBAADVAwAAhYYmFExJS0ozE8-rAI",
	"g_skip":       "CAADBAADYg4AAulsmFHxOkaz9OsTiwI",
	"b_0":          "CAADBAADVxUAAtnOkFEIAAGw5CZEIxgC",
	"b_1":          "CAADBAAD1RAAAnQqkFF9kDqD0wp3ngI",
	"b_2":          "CAADBAADZg4AAvcUmVHTXwldirf1hAI",
	"b_3":          "CAADBAADfBAAAkX1mVHw0CWX0h31iQI",
	"b_4":          "CAADBAADPBAAAuTCmFFDpvXzes4qjwI",
	"b_5":          "CAADBAADTQ4AAsWQmVHcrxDQUWOB4AI",
	"b_6":          "CAADBAAD_hAAAoUhmVG8kjd65J8EngI",
	"b_7":          "CAADBAADlRAAArtjkFGko5TuFNnncwI",
	"b_8":          "CAADBAADZQ8AAltEmFE_fDYIXBrV3QI",
	"b_9":          "CAADBAADrhAAAtM-mVGwhrWTD9IaYgI",
	"b_draw":       "CAADBAADtQ0AAnVbmFGC1hI60JaOQQI",
	"b_reverse":    "CAADBAADShEAAlcOmFHStPeFzfVIEwI",
	"b_skip":       "CAADBAAD_xEAAgZFmVFMRA1J8Y1gxAI",
	"y_0":          "CAADBAAD7xAAAqjjmFHnCu7eKJvSBgI",
	"y_1":          "CAADBAADJQwAAp6tmFE2zDPVMieQ2QI",
	"y_2":          "CAADBAADNA4AAl2mmVFpQOxJ41gk_gI",
	"y_3":          "CAADBAAD3A4AAsxPmFGyZFv42UlxAQI",
	"y_4":          "CAADBAADwg8AAm88kVEc9HZpl2gmzQI",
	"y_5":          "CAADBAAD5hIAAkQ6mFHS-aGVuYZAnAI",
	"y_6":          "CAADBAADvQ8AAs3RmVHVkVBfEF7eIwI",
	"y_7":          "CAADBAAD1gwAAjlbmFGGH6rBdqP8QQI",
	"y_8":          "CAADBAADbg8AAqvXkVH1ESeZFcGVrgI",
	"y_9":          "CAADBAADOQ8AAnjokVG96pmCP7aZ3AI",
	"y_draw":       "CAADBAAD6w4AAgsJmVETUteFwqTVJgI",
	"y_reverse":    "CAADBAADtg8AAqiFmFFwothyN9TrXwI",
	"y_skip":       "CAADBAADSxEAAhcSmFGu_F5LffmsZgI",
}

// CardToStickerKey maps an internal uno.Card to the string key used in the stickers map.
func CardToStickerKey(c uno.Card) string {
	if c.Rank == uno.Wild {
		return "colorchooser"
	}
	if c.Rank == uno.WildDrawFour {
		return "draw_four"
	}

	var colorPrefix string
	switch c.Color {
	case uno.Red:
		colorPrefix = "r"
	case uno.Green:
		colorPrefix = "g"
	case uno.Blue:
		colorPrefix = "b"
	case uno.Yellow:
		colorPrefix = "y"
	default:
		return ""
	}

	var rankSuffix string
	switch c.Rank {
	case uno.Zero:
		rankSuffix = "0"
	case uno.One:
		rankSuffix = "1"
	case uno.Two:
		rankSuffix = "2"
	case uno.Three:
		rankSuffix = "3"
	case uno.Four:
		rankSuffix = "4"
	case uno.Five:
		rankSuffix = "5"
	case uno.Six:
		rankSuffix = "6"
	case uno.Seven:
		rankSuffix = "7"
	case uno.Eight:
		rankSuffix = "8"
	case uno.Nine:
		rankSuffix = "9"
	case uno.DrawTwo:
		rankSuffix = "draw"
	case uno.Reverse:
		rankSuffix = "reverse"
	case uno.Skip:
		rankSuffix = "skip"
	default:
		return ""
	}

	return fmt.Sprintf("%s_%s", colorPrefix, rankSuffix)
}

func GetCardStickerID(c uno.Card) string {
	key := CardToStickerKey(c)
	return Stickers[key]
}

func GetCardStickerGreyID(c uno.Card) string {
	key := CardToStickerKey(c)
	return StickersGrey[key]
}
