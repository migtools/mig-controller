// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package language

<<<<<<< HEAD
import "golang.org/x/text/internal/language/compact"

=======
>>>>>>> cbc9bb05... fixup add vendor back
// TODO: Various sets of commonly use tags and regions.

// MustParse is like Parse, but panics if the given BCP 47 tag cannot be parsed.
// It simplifies safe initialization of Tag values.
func MustParse(s string) Tag {
	t, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return t
}

// MustParse is like Parse, but panics if the given BCP 47 tag cannot be parsed.
// It simplifies safe initialization of Tag values.
func (c CanonType) MustParse(s string) Tag {
	t, err := c.Parse(s)
	if err != nil {
		panic(err)
	}
	return t
}

// MustParseBase is like ParseBase, but panics if the given base cannot be parsed.
// It simplifies safe initialization of Base values.
func MustParseBase(s string) Base {
	b, err := ParseBase(s)
	if err != nil {
		panic(err)
	}
	return b
}

// MustParseScript is like ParseScript, but panics if the given script cannot be
// parsed. It simplifies safe initialization of Script values.
func MustParseScript(s string) Script {
	scr, err := ParseScript(s)
	if err != nil {
		panic(err)
	}
	return scr
}

// MustParseRegion is like ParseRegion, but panics if the given region cannot be
// parsed. It simplifies safe initialization of Region values.
func MustParseRegion(s string) Region {
	r, err := ParseRegion(s)
	if err != nil {
		panic(err)
	}
	return r
}

var (
	und = Tag{}

	Und Tag = Tag{}

<<<<<<< HEAD
	Afrikaans            Tag = Tag(compact.Afrikaans)
	Amharic              Tag = Tag(compact.Amharic)
	Arabic               Tag = Tag(compact.Arabic)
	ModernStandardArabic Tag = Tag(compact.ModernStandardArabic)
	Azerbaijani          Tag = Tag(compact.Azerbaijani)
	Bulgarian            Tag = Tag(compact.Bulgarian)
	Bengali              Tag = Tag(compact.Bengali)
	Catalan              Tag = Tag(compact.Catalan)
	Czech                Tag = Tag(compact.Czech)
	Danish               Tag = Tag(compact.Danish)
	German               Tag = Tag(compact.German)
	Greek                Tag = Tag(compact.Greek)
	English              Tag = Tag(compact.English)
	AmericanEnglish      Tag = Tag(compact.AmericanEnglish)
	BritishEnglish       Tag = Tag(compact.BritishEnglish)
	Spanish              Tag = Tag(compact.Spanish)
	EuropeanSpanish      Tag = Tag(compact.EuropeanSpanish)
	LatinAmericanSpanish Tag = Tag(compact.LatinAmericanSpanish)
	Estonian             Tag = Tag(compact.Estonian)
	Persian              Tag = Tag(compact.Persian)
	Finnish              Tag = Tag(compact.Finnish)
	Filipino             Tag = Tag(compact.Filipino)
	French               Tag = Tag(compact.French)
	CanadianFrench       Tag = Tag(compact.CanadianFrench)
	Gujarati             Tag = Tag(compact.Gujarati)
	Hebrew               Tag = Tag(compact.Hebrew)
	Hindi                Tag = Tag(compact.Hindi)
	Croatian             Tag = Tag(compact.Croatian)
	Hungarian            Tag = Tag(compact.Hungarian)
	Armenian             Tag = Tag(compact.Armenian)
	Indonesian           Tag = Tag(compact.Indonesian)
	Icelandic            Tag = Tag(compact.Icelandic)
	Italian              Tag = Tag(compact.Italian)
	Japanese             Tag = Tag(compact.Japanese)
	Georgian             Tag = Tag(compact.Georgian)
	Kazakh               Tag = Tag(compact.Kazakh)
	Khmer                Tag = Tag(compact.Khmer)
	Kannada              Tag = Tag(compact.Kannada)
	Korean               Tag = Tag(compact.Korean)
	Kirghiz              Tag = Tag(compact.Kirghiz)
	Lao                  Tag = Tag(compact.Lao)
	Lithuanian           Tag = Tag(compact.Lithuanian)
	Latvian              Tag = Tag(compact.Latvian)
	Macedonian           Tag = Tag(compact.Macedonian)
	Malayalam            Tag = Tag(compact.Malayalam)
	Mongolian            Tag = Tag(compact.Mongolian)
	Marathi              Tag = Tag(compact.Marathi)
	Malay                Tag = Tag(compact.Malay)
	Burmese              Tag = Tag(compact.Burmese)
	Nepali               Tag = Tag(compact.Nepali)
	Dutch                Tag = Tag(compact.Dutch)
	Norwegian            Tag = Tag(compact.Norwegian)
	Punjabi              Tag = Tag(compact.Punjabi)
	Polish               Tag = Tag(compact.Polish)
	Portuguese           Tag = Tag(compact.Portuguese)
	BrazilianPortuguese  Tag = Tag(compact.BrazilianPortuguese)
	EuropeanPortuguese   Tag = Tag(compact.EuropeanPortuguese)
	Romanian             Tag = Tag(compact.Romanian)
	Russian              Tag = Tag(compact.Russian)
	Sinhala              Tag = Tag(compact.Sinhala)
	Slovak               Tag = Tag(compact.Slovak)
	Slovenian            Tag = Tag(compact.Slovenian)
	Albanian             Tag = Tag(compact.Albanian)
	Serbian              Tag = Tag(compact.Serbian)
	SerbianLatin         Tag = Tag(compact.SerbianLatin)
	Swedish              Tag = Tag(compact.Swedish)
	Swahili              Tag = Tag(compact.Swahili)
	Tamil                Tag = Tag(compact.Tamil)
	Telugu               Tag = Tag(compact.Telugu)
	Thai                 Tag = Tag(compact.Thai)
	Turkish              Tag = Tag(compact.Turkish)
	Ukrainian            Tag = Tag(compact.Ukrainian)
	Urdu                 Tag = Tag(compact.Urdu)
	Uzbek                Tag = Tag(compact.Uzbek)
	Vietnamese           Tag = Tag(compact.Vietnamese)
	Chinese              Tag = Tag(compact.Chinese)
	SimplifiedChinese    Tag = Tag(compact.SimplifiedChinese)
	TraditionalChinese   Tag = Tag(compact.TraditionalChinese)
	Zulu                 Tag = Tag(compact.Zulu)
=======
	Afrikaans            Tag = Tag{lang: _af}                //  af
	Amharic              Tag = Tag{lang: _am}                //  am
	Arabic               Tag = Tag{lang: _ar}                //  ar
	ModernStandardArabic Tag = Tag{lang: _ar, region: _001}  //  ar-001
	Azerbaijani          Tag = Tag{lang: _az}                //  az
	Bulgarian            Tag = Tag{lang: _bg}                //  bg
	Bengali              Tag = Tag{lang: _bn}                //  bn
	Catalan              Tag = Tag{lang: _ca}                //  ca
	Czech                Tag = Tag{lang: _cs}                //  cs
	Danish               Tag = Tag{lang: _da}                //  da
	German               Tag = Tag{lang: _de}                //  de
	Greek                Tag = Tag{lang: _el}                //  el
	English              Tag = Tag{lang: _en}                //  en
	AmericanEnglish      Tag = Tag{lang: _en, region: _US}   //  en-US
	BritishEnglish       Tag = Tag{lang: _en, region: _GB}   //  en-GB
	Spanish              Tag = Tag{lang: _es}                //  es
	EuropeanSpanish      Tag = Tag{lang: _es, region: _ES}   //  es-ES
	LatinAmericanSpanish Tag = Tag{lang: _es, region: _419}  //  es-419
	Estonian             Tag = Tag{lang: _et}                //  et
	Persian              Tag = Tag{lang: _fa}                //  fa
	Finnish              Tag = Tag{lang: _fi}                //  fi
	Filipino             Tag = Tag{lang: _fil}               //  fil
	French               Tag = Tag{lang: _fr}                //  fr
	CanadianFrench       Tag = Tag{lang: _fr, region: _CA}   //  fr-CA
	Gujarati             Tag = Tag{lang: _gu}                //  gu
	Hebrew               Tag = Tag{lang: _he}                //  he
	Hindi                Tag = Tag{lang: _hi}                //  hi
	Croatian             Tag = Tag{lang: _hr}                //  hr
	Hungarian            Tag = Tag{lang: _hu}                //  hu
	Armenian             Tag = Tag{lang: _hy}                //  hy
	Indonesian           Tag = Tag{lang: _id}                //  id
	Icelandic            Tag = Tag{lang: _is}                //  is
	Italian              Tag = Tag{lang: _it}                //  it
	Japanese             Tag = Tag{lang: _ja}                //  ja
	Georgian             Tag = Tag{lang: _ka}                //  ka
	Kazakh               Tag = Tag{lang: _kk}                //  kk
	Khmer                Tag = Tag{lang: _km}                //  km
	Kannada              Tag = Tag{lang: _kn}                //  kn
	Korean               Tag = Tag{lang: _ko}                //  ko
	Kirghiz              Tag = Tag{lang: _ky}                //  ky
	Lao                  Tag = Tag{lang: _lo}                //  lo
	Lithuanian           Tag = Tag{lang: _lt}                //  lt
	Latvian              Tag = Tag{lang: _lv}                //  lv
	Macedonian           Tag = Tag{lang: _mk}                //  mk
	Malayalam            Tag = Tag{lang: _ml}                //  ml
	Mongolian            Tag = Tag{lang: _mn}                //  mn
	Marathi              Tag = Tag{lang: _mr}                //  mr
	Malay                Tag = Tag{lang: _ms}                //  ms
	Burmese              Tag = Tag{lang: _my}                //  my
	Nepali               Tag = Tag{lang: _ne}                //  ne
	Dutch                Tag = Tag{lang: _nl}                //  nl
	Norwegian            Tag = Tag{lang: _no}                //  no
	Punjabi              Tag = Tag{lang: _pa}                //  pa
	Polish               Tag = Tag{lang: _pl}                //  pl
	Portuguese           Tag = Tag{lang: _pt}                //  pt
	BrazilianPortuguese  Tag = Tag{lang: _pt, region: _BR}   //  pt-BR
	EuropeanPortuguese   Tag = Tag{lang: _pt, region: _PT}   //  pt-PT
	Romanian             Tag = Tag{lang: _ro}                //  ro
	Russian              Tag = Tag{lang: _ru}                //  ru
	Sinhala              Tag = Tag{lang: _si}                //  si
	Slovak               Tag = Tag{lang: _sk}                //  sk
	Slovenian            Tag = Tag{lang: _sl}                //  sl
	Albanian             Tag = Tag{lang: _sq}                //  sq
	Serbian              Tag = Tag{lang: _sr}                //  sr
	SerbianLatin         Tag = Tag{lang: _sr, script: _Latn} //  sr-Latn
	Swedish              Tag = Tag{lang: _sv}                //  sv
	Swahili              Tag = Tag{lang: _sw}                //  sw
	Tamil                Tag = Tag{lang: _ta}                //  ta
	Telugu               Tag = Tag{lang: _te}                //  te
	Thai                 Tag = Tag{lang: _th}                //  th
	Turkish              Tag = Tag{lang: _tr}                //  tr
	Ukrainian            Tag = Tag{lang: _uk}                //  uk
	Urdu                 Tag = Tag{lang: _ur}                //  ur
	Uzbek                Tag = Tag{lang: _uz}                //  uz
	Vietnamese           Tag = Tag{lang: _vi}                //  vi
	Chinese              Tag = Tag{lang: _zh}                //  zh
	SimplifiedChinese    Tag = Tag{lang: _zh, script: _Hans} //  zh-Hans
	TraditionalChinese   Tag = Tag{lang: _zh, script: _Hant} //  zh-Hant
	Zulu                 Tag = Tag{lang: _zu}                //  zu
>>>>>>> cbc9bb05... fixup add vendor back
)
