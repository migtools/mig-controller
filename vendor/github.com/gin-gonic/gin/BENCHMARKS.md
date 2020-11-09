
## Benchmark System

**VM HOST:** DigitalOcean  
<<<<<<< HEAD
**Machine:** 12 CPU, 24 GB RAM. Ubuntu 16.04.2 x64  
**Date:** Nov 26th, 2019  
**Go Version:** 1.13.4 linux/amd64  
**Source:** [Go HTTP Router Benchmark](https://github.com/julienschmidt/go-http-routing-benchmark)
**Result:** [See the gist](https://gist.github.com/appleboy/b5f2ecfaf50824ae9c64dcfb9165ae5e)
=======
**Machine:** 4 CPU, 8 GB RAM. Ubuntu 16.04.2 x64  
**Date:** July 19th, 2017  
**Go Version:** 1.8.3 linux/amd64  
**Source:** [Go HTTP Router Benchmark](https://github.com/julienschmidt/go-http-routing-benchmark)  
>>>>>>> cbc9bb05... fixup add vendor back

## Static Routes: 157

```
<<<<<<< HEAD
Gin:                 34936 Bytes

HttpServeMux:        14512 Bytes
Ace:                 30648 Bytes
Aero:               800696 Bytes
Bear:                30664 Bytes
Beego:               98456 Bytes
Bone:                40224 Bytes
Chi:                 83608 Bytes
CloudyKitRouter:     30448 Bytes
Denco:                9928 Bytes
Echo:                76584 Bytes
GocraftWeb:          55496 Bytes
Goji:                29744 Bytes
Gojiv2:             105840 Bytes
GoJsonRest:         137512 Bytes
GoRestful:          816936 Bytes
GorillaMux:         585632 Bytes
GowwwRouter:         24968 Bytes
HttpRouter:          21680 Bytes
HttpTreeMux:         73448 Bytes
Kocha:              115472 Bytes
LARS:                30640 Bytes
Macaron:             38592 Bytes
Martini:            310864 Bytes
Pat:                 19696 Bytes
Possum:              89920 Bytes
R2router:            23712 Bytes
Rivet:               24608 Bytes
Tango:               28264 Bytes
TigerTonic:          78768 Bytes
Traffic:            538976 Bytes
Vulcan:             369960 Bytes
=======
Gin:             30512 Bytes

HttpServeMux:    17344 Bytes
Ace:             30080 Bytes
Bear:            30472 Bytes
Beego:           96408 Bytes
Bone:            37904 Bytes
Denco:           10464 Bytes
Echo:            73680 Bytes
GocraftWeb:      55720 Bytes
Goji:            27200 Bytes
Gojiv2:         104464 Bytes
GoJsonRest:     136472 Bytes
GoRestful:      914904 Bytes
GorillaMux:     675568 Bytes
HttpRouter:      21128 Bytes
HttpTreeMux:     73448 Bytes
Kocha:          115072 Bytes
LARS:            30120 Bytes
Macaron:         37984 Bytes
Martini:        310832 Bytes
Pat:             20464 Bytes
Possum:          91328 Bytes
R2router:        23712 Bytes
Rivet:           23880 Bytes
Tango:           28008 Bytes
TigerTonic:      80368 Bytes
Traffic:        626480 Bytes
Vulcan:         369064 Bytes
>>>>>>> cbc9bb05... fixup add vendor back
```

## GithubAPI Routes: 203

```
<<<<<<< HEAD
Gin:                 58512 Bytes

Ace:                 48640 Bytes
Aero:              1386208 Bytes
Bear:                82536 Bytes
Beego:              150936 Bytes
Bone:               100976 Bytes
Chi:                 95112 Bytes
CloudyKitRouter:     93704 Bytes
Denco:               36736 Bytes
Echo:                96328 Bytes
GocraftWeb:          95432 Bytes
Goji:                51600 Bytes
Gojiv2:             104704 Bytes
GoJsonRest:         142024 Bytes
GoRestful:         1241656 Bytes
GorillaMux:        1322784 Bytes
GowwwRouter:         80008 Bytes
HttpRouter:          37096 Bytes
HttpTreeMux:         78800 Bytes
Kocha:              785408 Bytes
LARS:                48600 Bytes
Macaron:             93680 Bytes
Martini:            485264 Bytes
Pat:                 21200 Bytes
Possum:              85312 Bytes
R2router:            47104 Bytes
Rivet:               42840 Bytes
Tango:               54840 Bytes
TigerTonic:          96176 Bytes
Traffic:            921744 Bytes
Vulcan:             425368 Bytes
=======
Gin:             52672 Bytes

Ace:             48992 Bytes
Bear:           161592 Bytes
Beego:          147992 Bytes
Bone:            97728 Bytes
Denco:           36440 Bytes
Echo:            95672 Bytes
GocraftWeb:      95640 Bytes
Goji:            86088 Bytes
Gojiv2:         144392 Bytes
GoJsonRest:     134648 Bytes
GoRestful:     1410760 Bytes
GorillaMux:    1509488 Bytes
HttpRouter:      37464 Bytes
HttpTreeMux:     78800 Bytes
Kocha:          785408 Bytes
LARS:            49032 Bytes
Macaron:        132712 Bytes
Martini:        564352 Bytes
Pat:             21200 Bytes
Possum:          83888 Bytes
R2router:        47104 Bytes
Rivet:           42840 Bytes
Tango:           54584 Bytes
TigerTonic:      96384 Bytes
Traffic:       1061920 Bytes
Vulcan:         465296 Bytes
>>>>>>> cbc9bb05... fixup add vendor back
```

## GPlusAPI Routes: 13

```
<<<<<<< HEAD
Gin:              4384 Bytes

Ace: 3             664 Bytes
Aero:            88248 Bytes
Bear:             7112 Bytes
Beego:           10272 Bytes
Bone:             6688 Bytes
Chi:              8024 Bytes
CloudyKitRouter:  6728 Bytes
Denco:            3264 Bytes
Echo:             9272 Bytes
GocraftWeb:       7496 Bytes
Goji:             3152 Bytes
Gojiv2:           7376 Bytes
GoJsonRest:      11416 Bytes
GoRestful:       74328 Bytes
GorillaMux:      66208 Bytes
GowwwRouter:      5744 Bytes
HttpRouter:       2760 Bytes
HttpTreeMux:      7440 Bytes
Kocha:          128880 Bytes
LARS:             3656 Bytes
Macaron:          8656 Bytes
Martini:         23920 Bytes
=======
Gin:              3968 Bytes

Ace:              3600 Bytes
Bear:             7112 Bytes
Beego:           10048 Bytes
Bone:             6480 Bytes
Denco:            3256 Bytes
Echo:             9000 Bytes
GocraftWeb:       7496 Bytes
Goji:             2912 Bytes
Gojiv2:           7376 Bytes
GoJsonRest:      11544 Bytes
GoRestful:       88776 Bytes
GorillaMux:      71488 Bytes
HttpRouter:       2712 Bytes
HttpTreeMux:      7440 Bytes
Kocha:          128880 Bytes
LARS:             3640 Bytes
Macaron:          8656 Bytes
Martini:         23936 Bytes
>>>>>>> cbc9bb05... fixup add vendor back
Pat:              1856 Bytes
Possum:           7248 Bytes
R2router:         3928 Bytes
Rivet:            3064 Bytes
<<<<<<< HEAD
Tango:            5168 Bytes
TigerTonic:       9408 Bytes
Traffic:         46400 Bytes
Vulcan:          25544 Bytes
=======
Tango:            4912 Bytes
TigerTonic:       9408 Bytes
Traffic:         49472 Bytes
Vulcan:          25496 Bytes
>>>>>>> cbc9bb05... fixup add vendor back
```

## ParseAPI Routes: 26

```
<<<<<<< HEAD
Gin:              7776 Bytes

Ace:              6656 Bytes
Aero:           163736 Bytes
Bear:            12528 Bytes
Beego:           19280 Bytes
Bone:            11440 Bytes
Chi:              9744 Bytes
Denco:            4192 Bytes
Echo:            11648 Bytes
GocraftWeb:      12800 Bytes
Goji:             5680 Bytes
Gojiv2:          14464 Bytes
GoJsonRest:      14424 Bytes
GoRestful:      116264 Bytes
GorillaMux:     105880 Bytes
GowwwRouter:      9344 Bytes
HttpRouter:       5024 Bytes
=======
Gin:              6928 Bytes

Ace:              6592 Bytes
Bear:            12320 Bytes
Beego:           18960 Bytes
Bone:            11024 Bytes
Denco:            4184 Bytes
Echo:            11168 Bytes
GocraftWeb:      12800 Bytes
Goji:             5232 Bytes
Gojiv2:          14464 Bytes
GoJsonRest:      14216 Bytes
GoRestful:      127368 Bytes
GorillaMux:     123016 Bytes
HttpRouter:       4976 Bytes
>>>>>>> cbc9bb05... fixup add vendor back
HttpTreeMux:      7848 Bytes
Kocha:          181712 Bytes
LARS:             6632 Bytes
Macaron:         13648 Bytes
<<<<<<< HEAD
Martini:         45888 Bytes
=======
Martini:         45952 Bytes
>>>>>>> cbc9bb05... fixup add vendor back
Pat:              2560 Bytes
Possum:           9200 Bytes
R2router:         7056 Bytes
Rivet:            5680 Bytes
<<<<<<< HEAD
Tango:            8920 Bytes
TigerTonic:       9840 Bytes
Traffic:         79096 Bytes
=======
Tango:            8664 Bytes
TigerTonic:       9840 Bytes
Traffic:         93480 Bytes
>>>>>>> cbc9bb05... fixup add vendor back
Vulcan:          44504 Bytes
```

## Static Routes

```
<<<<<<< HEAD
BenchmarkGin_StaticAll                     25604             45487 ns/op               0 B/op          0 allocs/op

BenchmarkAce_StaticAll                     28402             42046 ns/op               0 B/op          0 allocs/op
BenchmarkAero_StaticAll                    38766             30333 ns/op               0 B/op          0 allocs/op
BenchmarkHttpServeMux_StaticAll            25728             46511 ns/op               0 B/op          0 allocs/op
BenchmarkBeego_StaticAll                    5098            288527 ns/op           55264 B/op        471 allocs/op
BenchmarkBear_StaticAll                    10000            126323 ns/op           20272 B/op        469 allocs/op
BenchmarkBone_StaticAll                     9499            113631 ns/op               0 B/op          0 allocs/op
BenchmarkChi_StaticAll                      7912            237363 ns/op           67824 B/op        471 allocs/op
BenchmarkCloudyKitRouter_StaticAll         41626             28668 ns/op               0 B/op          0 allocs/op
BenchmarkDenco_StaticAll                   95774             12221 ns/op               0 B/op          0 allocs/op
BenchmarkEcho_StaticAll                    26246             44603 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_StaticAll              10000            193337 ns/op           46312 B/op        785 allocs/op
BenchmarkGoji_StaticAll                    15886             75789 ns/op               0 B/op          0 allocs/op
BenchmarkGojiv2_StaticAll                   1886            597374 ns/op          205984 B/op       1570 allocs/op
BenchmarkGoJsonRest_StaticAll               4700            307144 ns/op           51653 B/op       1727 allocs/op
BenchmarkGoRestful_StaticAll                 429           2880165 ns/op          613280 B/op       2053 allocs/op
BenchmarkGorillaMux_StaticAll                754           1491761 ns/op          153233 B/op       1413 allocs/op
BenchmarkGowwwRouter_StaticAll             28071             42629 ns/op               0 B/op          0 allocs/op
BenchmarkHttpRouter_StaticAll              47672             24875 ns/op               0 B/op          0 allocs/op
BenchmarkHttpTreeMux_StaticAll             46770             25100 ns/op               0 B/op          0 allocs/op
BenchmarkKocha_StaticAll                   61045             19494 ns/op               0 B/op          0 allocs/op
BenchmarkLARS_StaticAll                    36103             32700 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_StaticAll                  4261            430131 ns/op          115552 B/op       1256 allocs/op
BenchmarkMartini_StaticAll                   481           2320157 ns/op          125444 B/op       1717 allocs/op
BenchmarkPat_StaticAll                       325           3739521 ns/op          602832 B/op      12559 allocs/op
BenchmarkPossum_StaticAll                  10000            203575 ns/op           65312 B/op        471 allocs/op
BenchmarkR2router_StaticAll                10000            110536 ns/op           22608 B/op        628 allocs/op
BenchmarkRivet_StaticAll                   23344             51174 ns/op               0 B/op          0 allocs/op
BenchmarkTango_StaticAll                    3596            340045 ns/op           39209 B/op       1256 allocs/op
BenchmarkTigerTonic_StaticAll              16784             71807 ns/op            7376 B/op        157 allocs/op
BenchmarkTraffic_StaticAll                   350           3435155 ns/op          754862 B/op      14601 allocs/op
BenchmarkVulcan_StaticAll                   5930            200284 ns/op           15386 B/op        471 allocs/op
=======
BenchmarkGin_StaticAll                     50000             34506 ns/op               0 B/op          0 allocs/op

BenchmarkAce_StaticAll                     30000             49657 ns/op               0 B/op          0 allocs/op
BenchmarkHttpServeMux_StaticAll             2000           1183737 ns/op              96 B/op          8 allocs/op
BenchmarkBeego_StaticAll                    5000            412621 ns/op           57776 B/op        628 allocs/op
BenchmarkBear_StaticAll                    10000            149242 ns/op           20336 B/op        461 allocs/op
BenchmarkBone_StaticAll                    10000            118583 ns/op               0 B/op          0 allocs/op
BenchmarkDenco_StaticAll                  100000             13247 ns/op               0 B/op          0 allocs/op
BenchmarkEcho_StaticAll                    20000             79914 ns/op            5024 B/op        157 allocs/op
BenchmarkGocraftWeb_StaticAll              10000            211823 ns/op           46440 B/op        785 allocs/op
BenchmarkGoji_StaticAll                    10000            109390 ns/op               0 B/op          0 allocs/op
BenchmarkGojiv2_StaticAll                   3000            415533 ns/op          145696 B/op       1099 allocs/op
BenchmarkGoJsonRest_StaticAll               5000            364403 ns/op           51653 B/op       1727 allocs/op
BenchmarkGoRestful_StaticAll                 500           2578579 ns/op          314936 B/op       3144 allocs/op
BenchmarkGorillaMux_StaticAll                500           2704856 ns/op          115648 B/op       1578 allocs/op
BenchmarkHttpRouter_StaticAll             100000             18541 ns/op               0 B/op          0 allocs/op
BenchmarkHttpTreeMux_StaticAll            100000             22332 ns/op               0 B/op          0 allocs/op
BenchmarkKocha_StaticAll                   50000             31176 ns/op               0 B/op          0 allocs/op
BenchmarkLARS_StaticAll                    50000             40840 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_StaticAll                  5000            517656 ns/op          120576 B/op       1413 allocs/op
BenchmarkMartini_StaticAll                   300           4462289 ns/op          125442 B/op       1717 allocs/op
BenchmarkPat_StaticAll                       500           2157275 ns/op          533904 B/op      11123 allocs/op
BenchmarkPossum_StaticAll                  10000            254701 ns/op           65312 B/op        471 allocs/op
BenchmarkR2router_StaticAll                10000            133956 ns/op           22608 B/op        628 allocs/op
BenchmarkRivet_StaticAll                   30000             46812 ns/op               0 B/op          0 allocs/op
BenchmarkTango_StaticAll                    5000            390613 ns/op           39225 B/op       1256 allocs/op
BenchmarkTigerTonic_StaticAll              20000             88060 ns/op            7504 B/op        157 allocs/op
BenchmarkTraffic_StaticAll                   500           2910236 ns/op          729736 B/op      14287 allocs/op
BenchmarkVulcan_StaticAll                   5000            277366 ns/op           15386 B/op        471 allocs/op
>>>>>>> cbc9bb05... fixup add vendor back
```

## Micro Benchmarks

```
<<<<<<< HEAD
BenchmarkGin_Param                       8623915               139 ns/op               0 B/op          0 allocs/op

BenchmarkAce_Param                       3976539               290 ns/op              32 B/op          1 allocs/op
BenchmarkAero_Param                      8948976               133 ns/op               0 B/op          0 allocs/op
BenchmarkBear_Param                      1000000              1277 ns/op             456 B/op          5 allocs/op
BenchmarkBeego_Param                      889404              1785 ns/op             352 B/op          3 allocs/op
BenchmarkBone_Param                      1000000              2219 ns/op             816 B/op          6 allocs/op
BenchmarkChi_Param                       1000000              1386 ns/op             432 B/op          3 allocs/op
BenchmarkCloudyKitRouter_Param          18343244                61.2 ns/op             0 B/op          0 allocs/op
BenchmarkDenco_Param                     5637424               204 ns/op              32 B/op          1 allocs/op
BenchmarkEcho_Param                      9540910               122 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_Param                1000000              1939 ns/op             648 B/op          8 allocs/op
BenchmarkGoji_Param                      1283509               938 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_Param                     331266              3554 ns/op            1328 B/op         11 allocs/op
BenchmarkGoJsonRest_Param                 908851              2158 ns/op             649 B/op         13 allocs/op
BenchmarkGoRestful_Param                  135781              9339 ns/op            4192 B/op         14 allocs/op
BenchmarkGorillaMux_Param                 308407              3893 ns/op            1280 B/op         10 allocs/op
BenchmarkGowwwRouter_Param               1000000              1044 ns/op             432 B/op          3 allocs/op
BenchmarkHttpRouter_Param                6653476               162 ns/op              32 B/op          1 allocs/op
BenchmarkHttpTreeMux_Param               1361378               819 ns/op             352 B/op          3 allocs/op
BenchmarkKocha_Param                     3084330               353 ns/op              56 B/op          3 allocs/op
BenchmarkLARS_Param                     11502079               107 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_Param                    439095              3750 ns/op            1072 B/op         10 allocs/op
BenchmarkMartini_Param                    177099              7479 ns/op            1072 B/op         10 allocs/op
BenchmarkPat_Param                        729747              2048 ns/op             536 B/op         11 allocs/op
BenchmarkPossum_Param                     995989              1705 ns/op             496 B/op          5 allocs/op
BenchmarkR2router_Param                  1000000              1037 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_Param                     4057065               271 ns/op              48 B/op          1 allocs/op
BenchmarkTango_Param                      812029              1682 ns/op             248 B/op          8 allocs/op
BenchmarkTigerTonic_Param                 450592              3358 ns/op             776 B/op         16 allocs/op
BenchmarkTraffic_Param                    206390              5661 ns/op            1856 B/op         21 allocs/op
BenchmarkVulcan_Param                    1441147               792 ns/op              98 B/op          3 allocs/op

BenchmarkAce_Param5                      1891473               632 ns/op             160 B/op          1 allocs/op
BenchmarkAero_Param5                     5191258               227 ns/op               0 B/op          0 allocs/op
BenchmarkBear_Param5                      988882              1734 ns/op             501 B/op          5 allocs/op
BenchmarkBeego_Param5                     625438              2132 ns/op             352 B/op          3 allocs/op
BenchmarkBone_Param5                      622030              3061 ns/op             864 B/op          6 allocs/op
BenchmarkChi_Param5                      1000000              1735 ns/op             432 B/op          3 allocs/op
BenchmarkCloudyKitRouter_Param5          5167868               225 ns/op               0 B/op          0 allocs/op
BenchmarkDenco_Param5                    2174550               550 ns/op             160 B/op          1 allocs/op
BenchmarkEcho_Param5                     4272258               275 ns/op               0 B/op          0 allocs/op
BenchmarkGin_Param5                      4190391               275 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_Param5                623739              3107 ns/op             920 B/op         11 allocs/op
BenchmarkGoji_Param5                     1000000              1310 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_Param5                    314694              3803 ns/op            1392 B/op         11 allocs/op
BenchmarkGoJsonRest_Param5                308203              4108 ns/op            1097 B/op         16 allocs/op
BenchmarkGoRestful_Param5                 115048              9787 ns/op            4288 B/op         14 allocs/op
BenchmarkGorillaMux_Param5                180812              5658 ns/op            1344 B/op         10 allocs/op
BenchmarkGowwwRouter_Param5              1000000              1156 ns/op             432 B/op          3 allocs/op
BenchmarkHttpRouter_Param5               2395767               502 ns/op             160 B/op          1 allocs/op
BenchmarkHttpTreeMux_Param5               899263              2096 ns/op             576 B/op          6 allocs/op
BenchmarkKocha_Param5                    1000000              1639 ns/op             440 B/op         10 allocs/op
BenchmarkLARS_Param5                     5807994               203 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_Param5                   272967              4087 ns/op            1072 B/op         10 allocs/op
BenchmarkMartini_Param5                   120735              8886 ns/op            1232 B/op         11 allocs/op
BenchmarkPat_Param5                       294714              4943 ns/op             888 B/op         29 allocs/op
BenchmarkPossum_Param5                   1000000              1689 ns/op             496 B/op          5 allocs/op
BenchmarkR2router_Param5                 1000000              1319 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_Param5                    1347289               883 ns/op             240 B/op          1 allocs/op
BenchmarkTango_Param5                     617077              2091 ns/op             360 B/op          8 allocs/op
BenchmarkTigerTonic_Param5                113659             11212 ns/op            2279 B/op         39 allocs/op
BenchmarkTraffic_Param5                   134148              9039 ns/op            2208 B/op         27 allocs/op
BenchmarkVulcan_Param5                   1000000              1095 ns/op              98 B/op          3 allocs/op

BenchmarkAce_Param20                     1000000              1838 ns/op             640 B/op          1 allocs/op
BenchmarkAero_Param20                   17120668                66.1 ns/op             0 B/op          0 allocs/op
BenchmarkBear_Param20                     205585              5332 ns/op            1665 B/op          5 allocs/op
BenchmarkBeego_Param20                    230522              5382 ns/op             352 B/op          3 allocs/op
BenchmarkBone_Param20                     167190              8076 ns/op            2031 B/op          6 allocs/op
BenchmarkChi_Param20                      480528              3044 ns/op             432 B/op          3 allocs/op
BenchmarkCloudyKitRouter_Param20         1347794               872 ns/op               0 B/op          0 allocs/op
BenchmarkDenco_Param20                   1000000              1867 ns/op             640 B/op          1 allocs/op
BenchmarkEcho_Param20                    1363526               897 ns/op               0 B/op          0 allocs/op
BenchmarkGin_Param20                     1607217               748 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_Param20                97314             11671 ns/op            3795 B/op         15 allocs/op
BenchmarkGoji_Param20                     289407              4220 ns/op            1246 B/op          2 allocs/op
BenchmarkGojiv2_Param20                   245186              4869 ns/op            1632 B/op         11 allocs/op
BenchmarkGoJsonRest_Param20                78049             15725 ns/op            4485 B/op         20 allocs/op
BenchmarkGoRestful_Param20                 66907             18031 ns/op            6716 B/op         18 allocs/op
BenchmarkGorillaMux_Param20                81866             12422 ns/op            3452 B/op         12 allocs/op
BenchmarkGowwwRouter_Param20              955983              1688 ns/op             432 B/op          3 allocs/op
BenchmarkHttpRouter_Param20              1000000              1629 ns/op             640 B/op          1 allocs/op
BenchmarkHttpTreeMux_Param20              108940             10241 ns/op            3195 B/op         10 allocs/op
BenchmarkKocha_Param20                    197022              5488 ns/op            1808 B/op         27 allocs/op
BenchmarkLARS_Param20                    2451241               490 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_Param20                  106770             10788 ns/op            2923 B/op         12 allocs/op
BenchmarkMartini_Param20                   69028             17112 ns/op            3596 B/op         13 allocs/op
BenchmarkPat_Param20                       56275             21535 ns/op            4424 B/op         93 allocs/op
BenchmarkPossum_Param20                  1000000              1705 ns/op             496 B/op          5 allocs/op
BenchmarkR2router_Param20                 172215              7099 ns/op            2283 B/op          7 allocs/op
BenchmarkRivet_Param20                    447265              2987 ns/op            1024 B/op          1 allocs/op
BenchmarkTango_Param20                    327494              3850 ns/op             856 B/op          8 allocs/op
BenchmarkTigerTonic_Param20                27176             44571 ns/op            9871 B/op        119 allocs/op
BenchmarkTraffic_Param20                   38828             31025 ns/op            7856 B/op         47 allocs/op
BenchmarkVulcan_Param20                   560442              1807 ns/op              98 B/op          3 allocs/op

BenchmarkAce_ParamWrite                  2712150               442 ns/op              40 B/op          2 allocs/op
BenchmarkAero_ParamWrite                 6392880               189 ns/op               0 B/op          0 allocs/op
BenchmarkBear_ParamWrite                 1000000              1338 ns/op             456 B/op          5 allocs/op
BenchmarkBeego_ParamWrite                 821431              1886 ns/op             360 B/op          4 allocs/op
BenchmarkBone_ParamWrite                  913227              2350 ns/op             816 B/op          6 allocs/op
BenchmarkChi_ParamWrite                  1000000              1427 ns/op             432 B/op          3 allocs/op
BenchmarkCloudyKitRouter_ParamWrite     18645724                60.9 ns/op             0 B/op          0 allocs/op
BenchmarkDenco_ParamWrite                4394764               264 ns/op              32 B/op          1 allocs/op
BenchmarkEcho_ParamWrite                 5288883               242 ns/op               8 B/op          1 allocs/op
BenchmarkGin_ParamWrite                  4584932               253 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_ParamWrite            866242              2094 ns/op             656 B/op          9 allocs/op
BenchmarkGoji_ParamWrite                 1201875              1004 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_ParamWrite                317766              3777 ns/op            1360 B/op         13 allocs/op
BenchmarkGoJsonRest_ParamWrite            380242              3447 ns/op            1128 B/op         18 allocs/op
BenchmarkGoRestful_ParamWrite             131046              9340 ns/op            4200 B/op         15 allocs/op
BenchmarkGorillaMux_ParamWrite            298428              3970 ns/op            1280 B/op         10 allocs/op
BenchmarkGowwwRouter_ParamWrite           655940              2744 ns/op             976 B/op          8 allocs/op
BenchmarkHttpRouter_ParamWrite           5237014               219 ns/op              32 B/op          1 allocs/op
BenchmarkHttpTreeMux_ParamWrite          1379904               853 ns/op             352 B/op          3 allocs/op
BenchmarkKocha_ParamWrite                2939042               400 ns/op              56 B/op          3 allocs/op
BenchmarkLARS_ParamWrite                 6181642               199 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_ParamWrite               352497              4670 ns/op            1176 B/op         14 allocs/op
BenchmarkMartini_ParamWrite               138259              8543 ns/op            1176 B/op         14 allocs/op
BenchmarkPat_ParamWrite                   552386              3262 ns/op             960 B/op         15 allocs/op
BenchmarkPossum_ParamWrite               1000000              1711 ns/op             496 B/op          5 allocs/op
BenchmarkR2router_ParamWrite             1000000              1085 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_ParamWrite                2374513               489 ns/op             112 B/op          2 allocs/op
BenchmarkTango_ParamWrite                1443907               812 ns/op             136 B/op          4 allocs/op
BenchmarkTigerTonic_ParamWrite            324264              4874 ns/op            1216 B/op         21 allocs/op
BenchmarkTraffic_ParamWrite               170726              7155 ns/op            2280 B/op         25 allocs/op
BenchmarkVulcan_ParamWrite               1498888               776 ns/op              98 B/op          3 allocs/op

=======
BenchmarkGin_Param                      20000000               113 ns/op               0 B/op          0 allocs/op

BenchmarkAce_Param                       5000000               375 ns/op              32 B/op          1 allocs/op
BenchmarkBear_Param                      1000000              1709 ns/op             456 B/op          5 allocs/op
BenchmarkBeego_Param                     1000000              2484 ns/op             368 B/op          4 allocs/op
BenchmarkBone_Param                      1000000              2391 ns/op             688 B/op          5 allocs/op
BenchmarkDenco_Param                    10000000               240 ns/op              32 B/op          1 allocs/op
BenchmarkEcho_Param                      5000000               366 ns/op              32 B/op          1 allocs/op
BenchmarkGocraftWeb_Param                1000000              2343 ns/op             648 B/op          8 allocs/op
BenchmarkGoji_Param                      1000000              1197 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_Param                    1000000              2771 ns/op             944 B/op          8 allocs/op
BenchmarkGoJsonRest_Param                1000000              2993 ns/op             649 B/op         13 allocs/op
BenchmarkGoRestful_Param                  200000              8860 ns/op            2296 B/op         21 allocs/op
BenchmarkGorillaMux_Param                 500000              4461 ns/op            1056 B/op         11 allocs/op
BenchmarkHttpRouter_Param               10000000               175 ns/op              32 B/op          1 allocs/op
BenchmarkHttpTreeMux_Param               1000000              1167 ns/op             352 B/op          3 allocs/op
BenchmarkKocha_Param                     3000000               429 ns/op              56 B/op          3 allocs/op
BenchmarkLARS_Param                     10000000               134 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_Param                    500000              4635 ns/op            1056 B/op         10 allocs/op
BenchmarkMartini_Param                    200000              9933 ns/op            1072 B/op         10 allocs/op
BenchmarkPat_Param                       1000000              2929 ns/op             648 B/op         12 allocs/op
BenchmarkPossum_Param                    1000000              2503 ns/op             560 B/op          6 allocs/op
BenchmarkR2router_Param                  1000000              1507 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_Param                     5000000               297 ns/op              48 B/op          1 allocs/op
BenchmarkTango_Param                     1000000              1862 ns/op             248 B/op          8 allocs/op
BenchmarkTigerTonic_Param                 500000              5660 ns/op             992 B/op         17 allocs/op
BenchmarkTraffic_Param                    200000              8408 ns/op            1960 B/op         21 allocs/op
BenchmarkVulcan_Param                    2000000               963 ns/op              98 B/op          3 allocs/op
BenchmarkAce_Param5                      2000000               740 ns/op             160 B/op          1 allocs/op
BenchmarkBear_Param5                     1000000              2777 ns/op             501 B/op          5 allocs/op
BenchmarkBeego_Param5                    1000000              3740 ns/op             368 B/op          4 allocs/op
BenchmarkBone_Param5                     1000000              2950 ns/op             736 B/op          5 allocs/op
BenchmarkDenco_Param5                    2000000               644 ns/op             160 B/op          1 allocs/op
BenchmarkEcho_Param5                     3000000               558 ns/op              32 B/op          1 allocs/op
BenchmarkGin_Param5                     10000000               198 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_Param5                500000              3870 ns/op             920 B/op         11 allocs/op
BenchmarkGoji_Param5                     1000000              1746 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_Param5                   1000000              3214 ns/op            1008 B/op          8 allocs/op
BenchmarkGoJsonRest_Param5                500000              5509 ns/op            1097 B/op         16 allocs/op
BenchmarkGoRestful_Param5                 200000             11232 ns/op            2392 B/op         21 allocs/op
BenchmarkGorillaMux_Param5                300000              7777 ns/op            1184 B/op         11 allocs/op
BenchmarkHttpRouter_Param5               3000000               631 ns/op             160 B/op          1 allocs/op
BenchmarkHttpTreeMux_Param5              1000000              2800 ns/op             576 B/op          6 allocs/op
BenchmarkKocha_Param5                    1000000              2053 ns/op             440 B/op         10 allocs/op
BenchmarkLARS_Param5                    10000000               232 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_Param5                   500000              5888 ns/op            1056 B/op         10 allocs/op
BenchmarkMartini_Param5                   200000             12807 ns/op            1232 B/op         11 allocs/op
BenchmarkPat_Param5                       300000              7320 ns/op             964 B/op         32 allocs/op
BenchmarkPossum_Param5                   1000000              2495 ns/op             560 B/op          6 allocs/op
BenchmarkR2router_Param5                 1000000              1844 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_Param5                    2000000               935 ns/op             240 B/op          1 allocs/op
BenchmarkTango_Param5                    1000000              2327 ns/op             360 B/op          8 allocs/op
BenchmarkTigerTonic_Param5                100000             18514 ns/op            2551 B/op         43 allocs/op
BenchmarkTraffic_Param5                   200000             11997 ns/op            2248 B/op         25 allocs/op
BenchmarkVulcan_Param5                   1000000              1333 ns/op              98 B/op          3 allocs/op
BenchmarkAce_Param20                     1000000              2031 ns/op             640 B/op          1 allocs/op
BenchmarkBear_Param20                     200000              7285 ns/op            1664 B/op          5 allocs/op
BenchmarkBeego_Param20                    300000              6224 ns/op             368 B/op          4 allocs/op
BenchmarkBone_Param20                     200000              8023 ns/op            1903 B/op          5 allocs/op
BenchmarkDenco_Param20                   1000000              2262 ns/op             640 B/op          1 allocs/op
BenchmarkEcho_Param20                    1000000              1387 ns/op              32 B/op          1 allocs/op
BenchmarkGin_Param20                     3000000               503 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_Param20               100000             14408 ns/op            3795 B/op         15 allocs/op
BenchmarkGoji_Param20                     500000              5272 ns/op            1247 B/op          2 allocs/op
BenchmarkGojiv2_Param20                  1000000              4163 ns/op            1248 B/op          8 allocs/op
BenchmarkGoJsonRest_Param20               100000             17866 ns/op            4485 B/op         20 allocs/op
BenchmarkGoRestful_Param20                100000             21022 ns/op            4724 B/op         23 allocs/op
BenchmarkGorillaMux_Param20               100000             17055 ns/op            3547 B/op         13 allocs/op
BenchmarkHttpRouter_Param20              1000000              1748 ns/op             640 B/op          1 allocs/op
BenchmarkHttpTreeMux_Param20              200000             12246 ns/op            3196 B/op         10 allocs/op
BenchmarkKocha_Param20                    300000              6861 ns/op            1808 B/op         27 allocs/op
BenchmarkLARS_Param20                    3000000               526 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_Param20                  100000             13069 ns/op            2906 B/op         12 allocs/op
BenchmarkMartini_Param20                  100000             23602 ns/op            3597 B/op         13 allocs/op
BenchmarkPat_Param20                       50000             32143 ns/op            4688 B/op        111 allocs/op
BenchmarkPossum_Param20                  1000000              2396 ns/op             560 B/op          6 allocs/op
BenchmarkR2router_Param20                 200000              8907 ns/op            2283 B/op          7 allocs/op
BenchmarkRivet_Param20                   1000000              3280 ns/op            1024 B/op          1 allocs/op
BenchmarkTango_Param20                    500000              4640 ns/op             856 B/op          8 allocs/op
BenchmarkTigerTonic_Param20                20000             67581 ns/op           10532 B/op        138 allocs/op
BenchmarkTraffic_Param20                   50000             40313 ns/op            7941 B/op         45 allocs/op
BenchmarkVulcan_Param20                  1000000              2264 ns/op              98 B/op          3 allocs/op
BenchmarkAce_ParamWrite                  3000000               532 ns/op              40 B/op          2 allocs/op
BenchmarkBear_ParamWrite                 1000000              1778 ns/op             456 B/op          5 allocs/op
BenchmarkBeego_ParamWrite                1000000              2596 ns/op             376 B/op          5 allocs/op
BenchmarkBone_ParamWrite                 1000000              2519 ns/op             688 B/op          5 allocs/op
BenchmarkDenco_ParamWrite                5000000               411 ns/op              32 B/op          1 allocs/op
BenchmarkEcho_ParamWrite                 2000000               718 ns/op              40 B/op          2 allocs/op
BenchmarkGin_ParamWrite                  5000000               283 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_ParamWrite           1000000              2561 ns/op             656 B/op          9 allocs/op
BenchmarkGoji_ParamWrite                 1000000              1378 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_ParamWrite               1000000              3128 ns/op             976 B/op         10 allocs/op
BenchmarkGoJsonRest_ParamWrite            500000              4446 ns/op            1128 B/op         18 allocs/op
BenchmarkGoRestful_ParamWrite             200000             10291 ns/op            2304 B/op         22 allocs/op
BenchmarkGorillaMux_ParamWrite            500000              5153 ns/op            1064 B/op         12 allocs/op
BenchmarkHttpRouter_ParamWrite           5000000               263 ns/op              32 B/op          1 allocs/op
BenchmarkHttpTreeMux_ParamWrite          1000000              1351 ns/op             352 B/op          3 allocs/op
BenchmarkKocha_ParamWrite                3000000               538 ns/op              56 B/op          3 allocs/op
BenchmarkLARS_ParamWrite                 5000000               316 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_ParamWrite               500000              5756 ns/op            1160 B/op         14 allocs/op
BenchmarkMartini_ParamWrite               200000             13097 ns/op            1176 B/op         14 allocs/op
BenchmarkPat_ParamWrite                   500000              4954 ns/op            1072 B/op         17 allocs/op
BenchmarkPossum_ParamWrite               1000000              2499 ns/op             560 B/op          6 allocs/op
BenchmarkR2router_ParamWrite             1000000              1531 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_ParamWrite                3000000               570 ns/op             112 B/op          2 allocs/op
BenchmarkTango_ParamWrite                2000000               957 ns/op             136 B/op          4 allocs/op
BenchmarkTigerTonic_ParamWrite            200000              7025 ns/op            1424 B/op         23 allocs/op
BenchmarkTraffic_ParamWrite               200000             10112 ns/op            2384 B/op         25 allocs/op
BenchmarkVulcan_ParamWrite               1000000              1006 ns/op              98 B/op          3 allocs/op
>>>>>>> cbc9bb05... fixup add vendor back
```

## GitHub

```
<<<<<<< HEAD
BenchmarkGin_GithubStatic                5866748               194 ns/op               0 B/op          0 allocs/op

BenchmarkAce_GithubStatic                5815826               205 ns/op               0 B/op          0 allocs/op
BenchmarkAero_GithubStatic              10822906               106 ns/op               0 B/op          0 allocs/op
BenchmarkBear_GithubStatic               1678065               707 ns/op             120 B/op          3 allocs/op
BenchmarkBeego_GithubStatic               828814              1717 ns/op             352 B/op          3 allocs/op
BenchmarkBone_GithubStatic                 67484             18858 ns/op            2880 B/op         60 allocs/op
BenchmarkCloudyKitRouter_GithubStatic   10219297               115 ns/op               0 B/op          0 allocs/op
BenchmarkChi_GithubStatic                1000000              1348 ns/op             432 B/op          3 allocs/op
BenchmarkDenco_GithubStatic             15220622                75.4 ns/op             0 B/op          0 allocs/op
BenchmarkEcho_GithubStatic               7255897               158 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GithubStatic         1000000              1198 ns/op             296 B/op          5 allocs/op
BenchmarkGoji_GithubStatic               3659361               320 ns/op               0 B/op          0 allocs/op
BenchmarkGojiv2_GithubStatic              402402              3384 ns/op            1312 B/op         10 allocs/op
BenchmarkGoRestful_GithubStatic            54592             22045 ns/op            4256 B/op         13 allocs/op
BenchmarkGoJsonRest_GithubStatic          801067              1673 ns/op             329 B/op         11 allocs/op
BenchmarkGorillaMux_GithubStatic          169690              8171 ns/op             976 B/op          9 allocs/op
BenchmarkGowwwRouter_GithubStatic        5372910               218 ns/op               0 B/op          0 allocs/op
BenchmarkHttpRouter_GithubStatic        10965576               103 ns/op               0 B/op          0 allocs/op
BenchmarkHttpTreeMux_GithubStatic       10505365               106 ns/op               0 B/op          0 allocs/op
BenchmarkKocha_GithubStatic             14153763                81.9 ns/op             0 B/op          0 allocs/op
BenchmarkLARS_GithubStatic               7874017               152 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GithubStatic             696940              2678 ns/op             736 B/op          8 allocs/op
BenchmarkMartini_GithubStatic             102384             12276 ns/op             768 B/op          9 allocs/op
BenchmarkPat_GithubStatic                  69907             17437 ns/op            3648 B/op         76 allocs/op
BenchmarkPossum_GithubStatic             1000000              1262 ns/op             416 B/op          3 allocs/op
BenchmarkR2router_GithubStatic           1981592               614 ns/op             144 B/op          4 allocs/op
BenchmarkRivet_GithubStatic              6103872               196 ns/op               0 B/op          0 allocs/op
BenchmarkTango_GithubStatic               629551              2023 ns/op             248 B/op          8 allocs/op
BenchmarkTigerTonic_GithubStatic         2801569               424 ns/op              48 B/op          1 allocs/op
BenchmarkTraffic_GithubStatic              63716             18009 ns/op            4664 B/op         90 allocs/op
BenchmarkVulcan_GithubStatic              885640              1177 ns/op              98 B/op          3 allocs/op

BenchmarkAce_GithubParam                 2016942               582 ns/op              96 B/op          1 allocs/op
BenchmarkAero_GithubParam                4009522               296 ns/op               0 B/op          0 allocs/op
BenchmarkBear_GithubParam                1000000              1575 ns/op             496 B/op          5 allocs/op
BenchmarkBeego_GithubParam                796662              2038 ns/op             352 B/op          3 allocs/op
BenchmarkBone_GithubParam                 114823             10325 ns/op            1888 B/op         19 allocs/op
BenchmarkChi_GithubParam                 1000000              1783 ns/op             432 B/op          3 allocs/op
BenchmarkCloudyKitRouter_GithubParam     3910996               303 ns/op               0 B/op          0 allocs/op
BenchmarkDenco_GithubParam               2298172               521 ns/op             128 B/op          1 allocs/op
BenchmarkEcho_GithubParam                3336364               357 ns/op               0 B/op          0 allocs/op
BenchmarkGin_GithubParam                 2729161               439 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GithubParam           825784              2338 ns/op             712 B/op          9 allocs/op
BenchmarkGoji_GithubParam                 933397              1559 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_GithubParam               253884              4335 ns/op            1408 B/op         13 allocs/op
BenchmarkGoJsonRest_GithubParam           575532              2967 ns/op             713 B/op         14 allocs/op
BenchmarkGoRestful_GithubParam             38160             30638 ns/op            4352 B/op         16 allocs/op
BenchmarkGorillaMux_GithubParam            94554             12035 ns/op            1296 B/op         10 allocs/op
BenchmarkGowwwRouter_GithubParam         1000000              1223 ns/op             432 B/op          3 allocs/op
BenchmarkHttpRouter_GithubParam          2562079               468 ns/op              96 B/op          1 allocs/op
BenchmarkHttpTreeMux_GithubParam         1000000              1386 ns/op             384 B/op          4 allocs/op
BenchmarkKocha_GithubParam               1573026               754 ns/op             128 B/op          5 allocs/op
BenchmarkLARS_GithubParam                4203394               282 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GithubParam              365078              4137 ns/op            1072 B/op         10 allocs/op
BenchmarkMartini_GithubParam               71608             15811 ns/op            1152 B/op         11 allocs/op
BenchmarkPat_GithubParam                   92768             13297 ns/op            2408 B/op         48 allocs/op
BenchmarkPossum_GithubParam              1000000              1704 ns/op             496 B/op          5 allocs/op
BenchmarkR2router_GithubParam            1000000              1120 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_GithubParam               1642794               720 ns/op              96 B/op          1 allocs/op
BenchmarkTango_GithubParam                574195              2345 ns/op             344 B/op          8 allocs/op
BenchmarkTigerTonic_GithubParam           272430              5493 ns/op            1176 B/op         22 allocs/op
BenchmarkTraffic_GithubParam               81914             15078 ns/op            2816 B/op         40 allocs/op
BenchmarkVulcan_GithubParam               581272              1902 ns/op              98 B/op          3 allocs/op


BenchmarkAce_GithubAll                     10000            103571 ns/op           13792 B/op        167 allocs/op
BenchmarkAero_GithubAll                    21366             55615 ns/op               0 B/op          0 allocs/op
BenchmarkBear_GithubAll                     5288            327648 ns/op           86448 B/op        943 allocs/op
BenchmarkBeego_GithubAll                    3974            413453 ns/op           71456 B/op        609 allocs/op
BenchmarkBone_GithubAll                      267           4450294 ns/op          720160 B/op       8620 allocs/op
BenchmarkChi_GithubAll                      5067            358773 ns/op           87696 B/op        609 allocs/op
BenchmarkCloudyKitRouter_GithubAll         24210             49233 ns/op               0 B/op          0 allocs/op
BenchmarkDenco_GithubAll                   12508             95341 ns/op           20224 B/op        167 allocs/op
BenchmarkEcho_GithubAll                    16353             73267 ns/op               0 B/op          0 allocs/op
BenchmarkGin_GithubAll                     15516             77716 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GithubAll               2908            466970 ns/op          131656 B/op       1686 allocs/op
BenchmarkGoji_GithubAll                     1746            691392 ns/op           56112 B/op        334 allocs/op
BenchmarkGojiv2_GithubAll                    954           1289604 ns/op          352720 B/op       4321 allocs/op
BenchmarkGoJsonRest_GithubAll               2013            599088 ns/op          134371 B/op       2737 allocs/op
BenchmarkGoRestful_GithubAll                 223           5404307 ns/op          910144 B/op       2938 allocs/op
BenchmarkGorillaMux_GithubAll                202           5943565 ns/op          251650 B/op       1994 allocs/op
BenchmarkGowwwRouter_GithubAll              9009            227799 ns/op           72144 B/op        501 allocs/op
BenchmarkHttpRouter_GithubAll              14570             78718 ns/op           13792 B/op        167 allocs/op
BenchmarkHttpTreeMux_GithubAll              7226            242491 ns/op           65856 B/op        671 allocs/op
BenchmarkKocha_GithubAll                    8282            159873 ns/op           23304 B/op        843 allocs/op
BenchmarkLARS_GithubAll                    22711             52745 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GithubAll                  2067            563117 ns/op          149409 B/op       1624 allocs/op
BenchmarkMartini_GithubAll                   218           5455290 ns/op          226552 B/op       2325 allocs/op
BenchmarkPat_GithubAll                       174           6801582 ns/op         1483152 B/op      26963 allocs/op
BenchmarkPossum_GithubAll                   8113            263665 ns/op           84448 B/op        609 allocs/op
BenchmarkR2router_GithubAll                 7172            247198 ns/op           77328 B/op        979 allocs/op
BenchmarkRivet_GithubAll                   10000            128086 ns/op           16272 B/op        167 allocs/op
BenchmarkTango_GithubAll                    3316            472753 ns/op           63825 B/op       1618 allocs/op
BenchmarkTigerTonic_GithubAll               1176           1041991 ns/op          193856 B/op       4474 allocs/op
BenchmarkTraffic_GithubAll                   226           5312082 ns/op          820742 B/op      14114 allocs/op
BenchmarkVulcan_GithubAll                   3904            304440 ns/op           19894 B/op        609 allocs/op
=======
BenchmarkGin_GithubStatic               10000000               156 ns/op               0 B/op          0 allocs/op

BenchmarkAce_GithubStatic                5000000               294 ns/op               0 B/op          0 allocs/op
BenchmarkBear_GithubStatic               2000000               893 ns/op             120 B/op          3 allocs/op
BenchmarkBeego_GithubStatic              1000000              2491 ns/op             368 B/op          4 allocs/op
BenchmarkBone_GithubStatic                 50000             25300 ns/op            2880 B/op         60 allocs/op
BenchmarkDenco_GithubStatic             20000000                76.0 ns/op             0 B/op          0 allocs/op
BenchmarkEcho_GithubStatic               2000000               516 ns/op              32 B/op          1 allocs/op
BenchmarkGocraftWeb_GithubStatic         1000000              1448 ns/op             296 B/op          5 allocs/op
BenchmarkGoji_GithubStatic               3000000               496 ns/op               0 B/op          0 allocs/op
BenchmarkGojiv2_GithubStatic             1000000              2941 ns/op             928 B/op          7 allocs/op
BenchmarkGoRestful_GithubStatic           100000             27256 ns/op            3224 B/op         22 allocs/op
BenchmarkGoJsonRest_GithubStatic         1000000              2196 ns/op             329 B/op         11 allocs/op
BenchmarkGorillaMux_GithubStatic           50000             31617 ns/op             736 B/op         10 allocs/op
BenchmarkHttpRouter_GithubStatic        20000000                88.4 ns/op             0 B/op          0 allocs/op
BenchmarkHttpTreeMux_GithubStatic       10000000               134 ns/op               0 B/op          0 allocs/op
BenchmarkKocha_GithubStatic             20000000               113 ns/op               0 B/op          0 allocs/op
BenchmarkLARS_GithubStatic              10000000               195 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GithubStatic             500000              3740 ns/op             768 B/op          9 allocs/op
BenchmarkMartini_GithubStatic              50000             27673 ns/op             768 B/op          9 allocs/op
BenchmarkPat_GithubStatic                 100000             19470 ns/op            3648 B/op         76 allocs/op
BenchmarkPossum_GithubStatic             1000000              1729 ns/op             416 B/op          3 allocs/op
BenchmarkR2router_GithubStatic           2000000               879 ns/op             144 B/op          4 allocs/op
BenchmarkRivet_GithubStatic             10000000               231 ns/op               0 B/op          0 allocs/op
BenchmarkTango_GithubStatic              1000000              2325 ns/op             248 B/op          8 allocs/op
BenchmarkTigerTonic_GithubStatic         3000000               610 ns/op              48 B/op          1 allocs/op
BenchmarkTraffic_GithubStatic              20000             62973 ns/op           18904 B/op        148 allocs/op
BenchmarkVulcan_GithubStatic             1000000              1447 ns/op              98 B/op          3 allocs/op
BenchmarkAce_GithubParam                 2000000               686 ns/op              96 B/op          1 allocs/op
BenchmarkBear_GithubParam                1000000              2155 ns/op             496 B/op          5 allocs/op
BenchmarkBeego_GithubParam               1000000              2713 ns/op             368 B/op          4 allocs/op
BenchmarkBone_GithubParam                 100000             15088 ns/op            1760 B/op         18 allocs/op
BenchmarkDenco_GithubParam               2000000               629 ns/op             128 B/op          1 allocs/op
BenchmarkEcho_GithubParam                2000000               653 ns/op              32 B/op          1 allocs/op
BenchmarkGin_GithubParam                 5000000               255 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GithubParam          1000000              3145 ns/op             712 B/op          9 allocs/op
BenchmarkGoji_GithubParam                1000000              1916 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_GithubParam              1000000              3975 ns/op            1024 B/op         10 allocs/op
BenchmarkGoJsonRest_GithubParam           300000              4134 ns/op             713 B/op         14 allocs/op
BenchmarkGoRestful_GithubParam             50000             30782 ns/op            2360 B/op         21 allocs/op
BenchmarkGorillaMux_GithubParam           100000             17148 ns/op            1088 B/op         11 allocs/op
BenchmarkHttpRouter_GithubParam          3000000               523 ns/op              96 B/op          1 allocs/op
BenchmarkHttpTreeMux_GithubParam         1000000              1671 ns/op             384 B/op          4 allocs/op
BenchmarkKocha_GithubParam               1000000              1021 ns/op             128 B/op          5 allocs/op
BenchmarkLARS_GithubParam                5000000               283 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GithubParam              500000              4270 ns/op            1056 B/op         10 allocs/op
BenchmarkMartini_GithubParam              100000             21728 ns/op            1152 B/op         11 allocs/op
BenchmarkPat_GithubParam                  200000             11208 ns/op            2464 B/op         48 allocs/op
BenchmarkPossum_GithubParam              1000000              2334 ns/op             560 B/op          6 allocs/op
BenchmarkR2router_GithubParam            1000000              1487 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_GithubParam               2000000               782 ns/op              96 B/op          1 allocs/op
BenchmarkTango_GithubParam               1000000              2653 ns/op             344 B/op          8 allocs/op
BenchmarkTigerTonic_GithubParam           300000             14073 ns/op            1440 B/op         24 allocs/op
BenchmarkTraffic_GithubParam               50000             29164 ns/op            5992 B/op         52 allocs/op
BenchmarkVulcan_GithubParam              1000000              2529 ns/op              98 B/op          3 allocs/op
BenchmarkAce_GithubAll                     10000            134059 ns/op           13792 B/op        167 allocs/op
BenchmarkBear_GithubAll                     5000            534445 ns/op           86448 B/op        943 allocs/op
BenchmarkBeego_GithubAll                    3000            592444 ns/op           74705 B/op        812 allocs/op
BenchmarkBone_GithubAll                      200           6957308 ns/op          698784 B/op       8453 allocs/op
BenchmarkDenco_GithubAll                   10000            158819 ns/op           20224 B/op        167 allocs/op
BenchmarkEcho_GithubAll                    10000            154700 ns/op            6496 B/op        203 allocs/op
BenchmarkGin_GithubAll                     30000             48375 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GithubAll               3000            570806 ns/op          131656 B/op       1686 allocs/op
BenchmarkGoji_GithubAll                     2000            818034 ns/op           56112 B/op        334 allocs/op
BenchmarkGojiv2_GithubAll                   2000           1213973 ns/op          274768 B/op       3712 allocs/op
BenchmarkGoJsonRest_GithubAll               2000            785796 ns/op          134371 B/op       2737 allocs/op
BenchmarkGoRestful_GithubAll                 300           5238188 ns/op          689672 B/op       4519 allocs/op
BenchmarkGorillaMux_GithubAll                100          10257726 ns/op          211840 B/op       2272 allocs/op
BenchmarkHttpRouter_GithubAll              20000            105414 ns/op           13792 B/op        167 allocs/op
BenchmarkHttpTreeMux_GithubAll             10000            319934 ns/op           65856 B/op        671 allocs/op
BenchmarkKocha_GithubAll                   10000            209442 ns/op           23304 B/op        843 allocs/op
BenchmarkLARS_GithubAll                    20000             62565 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GithubAll                  2000           1161270 ns/op          204194 B/op       2000 allocs/op
BenchmarkMartini_GithubAll                   200           9991713 ns/op          226549 B/op       2325 allocs/op
BenchmarkPat_GithubAll                       200           5590793 ns/op         1499568 B/op      27435 allocs/op
BenchmarkPossum_GithubAll                  10000            319768 ns/op           84448 B/op        609 allocs/op
BenchmarkR2router_GithubAll                10000            305134 ns/op           77328 B/op        979 allocs/op
BenchmarkRivet_GithubAll                   10000            132134 ns/op           16272 B/op        167 allocs/op
BenchmarkTango_GithubAll                    3000            552754 ns/op           63826 B/op       1618 allocs/op
BenchmarkTigerTonic_GithubAll               1000           1439483 ns/op          239104 B/op       5374 allocs/op
BenchmarkTraffic_GithubAll                   100          11383067 ns/op         2659329 B/op      21848 allocs/op
BenchmarkVulcan_GithubAll                   5000            394253 ns/op           19894 B/op        609 allocs/op
>>>>>>> cbc9bb05... fixup add vendor back
```

## Google+

```
<<<<<<< HEAD
BenchmarkGin_GPlusStatic                 9172405               124 ns/op               0 B/op          0 allocs/op

BenchmarkAce_GPlusStatic                 7784710               152 ns/op               0 B/op          0 allocs/op
BenchmarkAero_GPlusStatic               12771894                89.2 ns/op             0 B/op          0 allocs/op
BenchmarkBear_GPlusStatic                2351325               512 ns/op             104 B/op          3 allocs/op
BenchmarkBeego_GPlusStatic               1000000              1643 ns/op             352 B/op          3 allocs/op
BenchmarkBone_GPlusStatic                4419217               263 ns/op              32 B/op          1 allocs/op
BenchmarkChi_GPlusStatic                 1000000              1282 ns/op             432 B/op          3 allocs/op
BenchmarkCloudyKitRouter_GPlusStatic    17730754                61.9 ns/op             0 B/op          0 allocs/op
BenchmarkDenco_GPlusStatic              29549895                38.3 ns/op             0 B/op          0 allocs/op
BenchmarkEcho_GPlusStatic               10521789               111 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GPlusStatic          1000000              1053 ns/op             280 B/op          5 allocs/op
BenchmarkGoji_GPlusStatic                5209968               228 ns/op               0 B/op          0 allocs/op
BenchmarkGojiv2_GPlusStatic               306363              3348 ns/op            1312 B/op         10 allocs/op
BenchmarkGoJsonRest_GPlusStatic          1000000              1424 ns/op             329 B/op         11 allocs/op
BenchmarkGoRestful_GPlusStatic            130754              8760 ns/op            3872 B/op         13 allocs/op
BenchmarkGorillaMux_GPlusStatic           496250              2860 ns/op             976 B/op          9 allocs/op
BenchmarkGowwwRouter_GPlusStatic        16401519                66.5 ns/op             0 B/op          0 allocs/op
BenchmarkHttpRouter_GPlusStatic         21323139                50.3 ns/op             0 B/op          0 allocs/op
BenchmarkHttpTreeMux_GPlusStatic        14877926                68.7 ns/op             0 B/op          0 allocs/op
BenchmarkKocha_GPlusStatic              18375128                57.6 ns/op             0 B/op          0 allocs/op
BenchmarkLARS_GPlusStatic               11153810               101 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GPlusStatic              652598              2720 ns/op             736 B/op          8 allocs/op
BenchmarkMartini_GPlusStatic              218824              6532 ns/op             768 B/op          9 allocs/op
BenchmarkPat_GPlusStatic                 2825560               428 ns/op              96 B/op          2 allocs/op
BenchmarkPossum_GPlusStatic              1000000              1236 ns/op             416 B/op          3 allocs/op
BenchmarkR2router_GPlusStatic            2222193               541 ns/op             144 B/op          4 allocs/op
BenchmarkRivet_GPlusStatic               9802023               114 ns/op               0 B/op          0 allocs/op
BenchmarkTango_GPlusStatic                980658              1465 ns/op             200 B/op          8 allocs/op
BenchmarkTigerTonic_GPlusStatic          4882701               239 ns/op              32 B/op          1 allocs/op
BenchmarkTraffic_GPlusStatic              508060              3465 ns/op            1112 B/op         16 allocs/op
BenchmarkVulcan_GPlusStatic              1608979               725 ns/op              98 B/op          3 allocs/op

BenchmarkAce_GPlusParam                  2962957               414 ns/op              64 B/op          1 allocs/op
BenchmarkAero_GPlusParam                 5667668               202 ns/op               0 B/op          0 allocs/op
BenchmarkBear_GPlusParam                 1000000              1271 ns/op             480 B/op          5 allocs/op
BenchmarkBeego_GPlusParam                 869858              1874 ns/op             352 B/op          3 allocs/op
BenchmarkBone_GPlusParam                  869476              2395 ns/op             816 B/op          6 allocs/op
BenchmarkChi_GPlusParam                  1000000              1469 ns/op             432 B/op          3 allocs/op
BenchmarkCloudyKitRouter_GPlusParam     11149783               108 ns/op               0 B/op          0 allocs/op
BenchmarkDenco_GPlusParam                4007298               301 ns/op              64 B/op          1 allocs/op
BenchmarkEcho_GPlusParam                 6448201               174 ns/op               0 B/op          0 allocs/op
BenchmarkGin_GPlusParam                  5470827               218 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GPlusParam           1000000              1939 ns/op             648 B/op          8 allocs/op
BenchmarkGoji_GPlusParam                 1207621               997 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_GPlusParam                271326              4013 ns/op            1328 B/op         11 allocs/op
BenchmarkGoJsonRest_GPlusParam            781062              2303 ns/op             649 B/op         13 allocs/op
BenchmarkGoRestful_GPlusParam             121267              9871 ns/op            4192 B/op         14 allocs/op
BenchmarkGorillaMux_GPlusParam            228406              5156 ns/op            1280 B/op         10 allocs/op
BenchmarkGowwwRouter_GPlusParam          1000000              1074 ns/op             432 B/op          3 allocs/op
BenchmarkHttpRouter_GPlusParam           4399740               276 ns/op              64 B/op          1 allocs/op
BenchmarkHttpTreeMux_GPlusParam          1309540               898 ns/op             352 B/op          3 allocs/op
BenchmarkKocha_GPlusParam                2930965               403 ns/op              56 B/op          3 allocs/op
BenchmarkLARS_GPlusParam                 7588237               151 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GPlusParam               434997              4195 ns/op            1072 B/op         10 allocs/op
BenchmarkMartini_GPlusParam               148207              8144 ns/op            1072 B/op         10 allocs/op
BenchmarkPat_GPlusParam                   566829              2533 ns/op             576 B/op         11 allocs/op
BenchmarkPossum_GPlusParam               1000000              1723 ns/op             496 B/op          5 allocs/op
BenchmarkR2router_GPlusParam             1000000              1100 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_GPlusParam                3309052               331 ns/op              48 B/op          1 allocs/op
BenchmarkTango_GPlusParam                 693728              1825 ns/op             264 B/op          8 allocs/op
BenchmarkTigerTonic_GPlusParam            417693              3800 ns/op             856 B/op         16 allocs/op
BenchmarkTraffic_GPlusParam               179424              6641 ns/op            1872 B/op         21 allocs/op
BenchmarkVulcan_GPlusParam               1000000              1063 ns/op              98 B/op          3 allocs/op

BenchmarkAce_GPlus2Params                2720149               460 ns/op              64 B/op          1 allocs/op
BenchmarkAero_GPlus2Params               3525165               343 ns/op               0 B/op          0 allocs/op
BenchmarkBear_GPlus2Params               1000000              1502 ns/op             496 B/op          5 allocs/op
BenchmarkBeego_GPlus2Params               730123              2102 ns/op             352 B/op          3 allocs/op
BenchmarkBone_GPlus2Params                253177              5583 ns/op            1168 B/op         10 allocs/op
BenchmarkChi_GPlus2Params                1000000              1531 ns/op             432 B/op          3 allocs/op
BenchmarkCloudyKitRouter_GPlus2Params    6943176               168 ns/op               0 B/op          0 allocs/op
BenchmarkDenco_GPlus2Params              2912601               413 ns/op              64 B/op          1 allocs/op
BenchmarkEcho_GPlus2Params               4149189               278 ns/op               0 B/op          0 allocs/op
BenchmarkGin_GPlus2Params                3271269               356 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GPlus2Params          915531              2321 ns/op             712 B/op          9 allocs/op
BenchmarkGoji_GPlus2Params               1000000              1413 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_GPlus2Params              256640              4521 ns/op            1408 B/op         14 allocs/op
BenchmarkGoJsonRest_GPlus2Params          499140              3076 ns/op             713 B/op         14 allocs/op
BenchmarkGoRestful_GPlus2Params           105928             10148 ns/op            4384 B/op         16 allocs/op
BenchmarkGorillaMux_GPlus2Params          110953              9682 ns/op            1296 B/op         10 allocs/op
BenchmarkGowwwRouter_GPlus2Params        1000000              1112 ns/op             432 B/op          3 allocs/op
BenchmarkHttpRouter_GPlus2Params         3491893               321 ns/op              64 B/op          1 allocs/op
BenchmarkHttpTreeMux_GPlus2Params        1000000              1341 ns/op             384 B/op          4 allocs/op
BenchmarkKocha_GPlus2Params              1445288               790 ns/op             128 B/op          5 allocs/op
BenchmarkLARS_GPlus2Params               6644953               185 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GPlus2Params             424291              4321 ns/op            1072 B/op         10 allocs/op
BenchmarkMartini_GPlus2Params              70866             16407 ns/op            1200 B/op         13 allocs/op
BenchmarkPat_GPlus2Params                 121308             10221 ns/op            2168 B/op         33 allocs/op
BenchmarkPossum_GPlus2Params             1000000              1847 ns/op             496 B/op          5 allocs/op
BenchmarkR2router_GPlus2Params           1000000              1267 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_GPlus2Params              2017526               590 ns/op              96 B/op          1 allocs/op
BenchmarkTango_GPlus2Params               846003              2143 ns/op             344 B/op          8 allocs/op
BenchmarkTigerTonic_GPlus2Params          303597              5736 ns/op            1200 B/op         22 allocs/op
BenchmarkTraffic_GPlus2Params              95032             12817 ns/op            2248 B/op         28 allocs/op
BenchmarkVulcan_GPlus2Params              692610              1575 ns/op              98 B/op          3 allocs/op

BenchmarkAce_GPlusAll                     271720              4948 ns/op             640 B/op         11 allocs/op
BenchmarkAero_GPlusAll                    367956              2926 ns/op               0 B/op          0 allocs/op
BenchmarkBear_GPlusAll                     68161             17883 ns/op            5488 B/op         61 allocs/op
BenchmarkBeego_GPlusAll                    46634             25369 ns/op            4576 B/op         39 allocs/op
BenchmarkBone_GPlusAll                     24628             49198 ns/op           11744 B/op        109 allocs/op
BenchmarkChi_GPlusAll                      60778             19356 ns/op            5616 B/op         39 allocs/op
BenchmarkCloudyKitRouter_GPlusAll         706952              1693 ns/op               0 B/op          0 allocs/op
BenchmarkDenco_GPlusAll                   327422              4222 ns/op             672 B/op         11 allocs/op
BenchmarkEcho_GPlusAll                    331987              3176 ns/op               0 B/op          0 allocs/op
BenchmarkGin_GPlusAll                     289526              3559 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GPlusAll               45805             26768 ns/op            8040 B/op        103 allocs/op
BenchmarkGoji_GPlusAll                     74786             14428 ns/op            3696 B/op         22 allocs/op
BenchmarkGojiv2_GPlusAll                   23822             50355 ns/op           17616 B/op        154 allocs/op
BenchmarkGoJsonRest_GPlusAll               35280             32989 ns/op            8117 B/op        170 allocs/op
BenchmarkGoRestful_GPlusAll                10000            129418 ns/op           55520 B/op        192 allocs/op
BenchmarkGorillaMux_GPlusAll               15968             76492 ns/op           16112 B/op        128 allocs/op
BenchmarkGowwwRouter_GPlusAll             100096             12644 ns/op            4752 B/op         33 allocs/op
BenchmarkHttpRouter_GPlusAll              474584              3704 ns/op             640 B/op         11 allocs/op
BenchmarkHttpTreeMux_GPlusAll              98506             12480 ns/op            4032 B/op         38 allocs/op
BenchmarkKocha_GPlusAll                   213709              7358 ns/op             976 B/op         43 allocs/op
BenchmarkLARS_GPlusAll                    466608              2363 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GPlusAll                  34136             35790 ns/op            9568 B/op        104 allocs/op
BenchmarkMartini_GPlusAll                   8911            124543 ns/op           14016 B/op        145 allocs/op
BenchmarkPat_GPlusAll                      17391             69198 ns/op           15264 B/op        271 allocs/op
BenchmarkPossum_GPlusAll                   66774             17004 ns/op            5408 B/op         39 allocs/op
BenchmarkR2router_GPlusAll                 79681             13996 ns/op            5040 B/op         63 allocs/op
BenchmarkRivet_GPlusAll                   258788              5344 ns/op             768 B/op         11 allocs/op
BenchmarkTango_GPlusAll                    46930             25591 ns/op            3656 B/op        104 allocs/op
BenchmarkTigerTonic_GPlusAll               20768             58038 ns/op           11600 B/op        242 allocs/op
BenchmarkTraffic_GPlusAll                  10000            108031 ns/op           26248 B/op        341 allocs/op
BenchmarkVulcan_GPlusAll                   71826             15724 ns/op            1274 B/op         39 allocs/op
=======
BenchmarkGin_GPlusStatic                10000000               183 ns/op               0 B/op          0 allocs/op

BenchmarkAce_GPlusStatic                 5000000               276 ns/op               0 B/op          0 allocs/op
BenchmarkBear_GPlusStatic                2000000               652 ns/op             104 B/op          3 allocs/op
BenchmarkBeego_GPlusStatic               1000000              2239 ns/op             368 B/op          4 allocs/op
BenchmarkBone_GPlusStatic                5000000               380 ns/op              32 B/op          1 allocs/op
BenchmarkDenco_GPlusStatic              30000000                45.8 ns/op             0 B/op          0 allocs/op
BenchmarkEcho_GPlusStatic                5000000               338 ns/op              32 B/op          1 allocs/op
BenchmarkGocraftWeb_GPlusStatic          1000000              1158 ns/op             280 B/op          5 allocs/op
BenchmarkGoji_GPlusStatic                5000000               331 ns/op               0 B/op          0 allocs/op
BenchmarkGojiv2_GPlusStatic              1000000              2106 ns/op             928 B/op          7 allocs/op
BenchmarkGoJsonRest_GPlusStatic          1000000              1626 ns/op             329 B/op         11 allocs/op
BenchmarkGoRestful_GPlusStatic            300000              7598 ns/op            1976 B/op         20 allocs/op
BenchmarkGorillaMux_GPlusStatic          1000000              2629 ns/op             736 B/op         10 allocs/op
BenchmarkHttpRouter_GPlusStatic         30000000                52.5 ns/op             0 B/op          0 allocs/op
BenchmarkHttpTreeMux_GPlusStatic        20000000                85.8 ns/op             0 B/op          0 allocs/op
BenchmarkKocha_GPlusStatic              20000000                89.2 ns/op             0 B/op          0 allocs/op
BenchmarkLARS_GPlusStatic               10000000               162 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GPlusStatic              500000              3479 ns/op             768 B/op          9 allocs/op
BenchmarkMartini_GPlusStatic              200000              9092 ns/op             768 B/op          9 allocs/op
BenchmarkPat_GPlusStatic                 3000000               493 ns/op              96 B/op          2 allocs/op
BenchmarkPossum_GPlusStatic              1000000              1467 ns/op             416 B/op          3 allocs/op
BenchmarkR2router_GPlusStatic            2000000               788 ns/op             144 B/op          4 allocs/op
BenchmarkRivet_GPlusStatic              20000000               114 ns/op               0 B/op          0 allocs/op
BenchmarkTango_GPlusStatic               1000000              1534 ns/op             200 B/op          8 allocs/op
BenchmarkTigerTonic_GPlusStatic          5000000               282 ns/op              32 B/op          1 allocs/op
BenchmarkTraffic_GPlusStatic              500000              3798 ns/op            1192 B/op         15 allocs/op
BenchmarkVulcan_GPlusStatic              2000000              1125 ns/op              98 B/op          3 allocs/op
BenchmarkAce_GPlusParam                  3000000               528 ns/op              64 B/op          1 allocs/op
BenchmarkBear_GPlusParam                 1000000              1570 ns/op             480 B/op          5 allocs/op
BenchmarkBeego_GPlusParam                1000000              2369 ns/op             368 B/op          4 allocs/op
BenchmarkBone_GPlusParam                 1000000              2028 ns/op             688 B/op          5 allocs/op
BenchmarkDenco_GPlusParam                5000000               385 ns/op              64 B/op          1 allocs/op
BenchmarkEcho_GPlusParam                 3000000               441 ns/op              32 B/op          1 allocs/op
BenchmarkGin_GPlusParam                 10000000               174 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GPlusParam           1000000              2033 ns/op             648 B/op          8 allocs/op
BenchmarkGoji_GPlusParam                 1000000              1399 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_GPlusParam               1000000              2641 ns/op             944 B/op          8 allocs/op
BenchmarkGoJsonRest_GPlusParam           1000000              2824 ns/op             649 B/op         13 allocs/op
BenchmarkGoRestful_GPlusParam             200000              8875 ns/op            2296 B/op         21 allocs/op
BenchmarkGorillaMux_GPlusParam            200000              6291 ns/op            1056 B/op         11 allocs/op
BenchmarkHttpRouter_GPlusParam           5000000               316 ns/op              64 B/op          1 allocs/op
BenchmarkHttpTreeMux_GPlusParam          1000000              1129 ns/op             352 B/op          3 allocs/op
BenchmarkKocha_GPlusParam                3000000               538 ns/op              56 B/op          3 allocs/op
BenchmarkLARS_GPlusParam                10000000               198 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GPlusParam               500000              3554 ns/op            1056 B/op         10 allocs/op
BenchmarkMartini_GPlusParam               200000              9831 ns/op            1072 B/op         10 allocs/op
BenchmarkPat_GPlusParam                  1000000              2706 ns/op             688 B/op         12 allocs/op
BenchmarkPossum_GPlusParam               1000000              2297 ns/op             560 B/op          6 allocs/op
BenchmarkR2router_GPlusParam             1000000              1318 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_GPlusParam                5000000               399 ns/op              48 B/op          1 allocs/op
BenchmarkTango_GPlusParam                1000000              2070 ns/op             264 B/op          8 allocs/op
BenchmarkTigerTonic_GPlusParam            500000              4853 ns/op            1056 B/op         17 allocs/op
BenchmarkTraffic_GPlusParam               200000              8278 ns/op            1976 B/op         21 allocs/op
BenchmarkVulcan_GPlusParam               1000000              1243 ns/op              98 B/op          3 allocs/op
BenchmarkAce_GPlus2Params                3000000               549 ns/op              64 B/op          1 allocs/op
BenchmarkBear_GPlus2Params               1000000              2112 ns/op             496 B/op          5 allocs/op
BenchmarkBeego_GPlus2Params               500000              2750 ns/op             368 B/op          4 allocs/op
BenchmarkBone_GPlus2Params                300000              7032 ns/op            1040 B/op          9 allocs/op
BenchmarkDenco_GPlus2Params              3000000               502 ns/op              64 B/op          1 allocs/op
BenchmarkEcho_GPlus2Params               3000000               641 ns/op              32 B/op          1 allocs/op
BenchmarkGin_GPlus2Params                5000000               250 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GPlus2Params         1000000              2681 ns/op             712 B/op          9 allocs/op
BenchmarkGoji_GPlus2Params               1000000              1926 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_GPlus2Params              500000              3996 ns/op            1024 B/op         11 allocs/op
BenchmarkGoJsonRest_GPlus2Params          500000              3886 ns/op             713 B/op         14 allocs/op
BenchmarkGoRestful_GPlus2Params           200000             10376 ns/op            2360 B/op         21 allocs/op
BenchmarkGorillaMux_GPlus2Params          100000             14162 ns/op            1088 B/op         11 allocs/op
BenchmarkHttpRouter_GPlus2Params         5000000               336 ns/op              64 B/op          1 allocs/op
BenchmarkHttpTreeMux_GPlus2Params        1000000              1523 ns/op             384 B/op          4 allocs/op
BenchmarkKocha_GPlus2Params              2000000               970 ns/op             128 B/op          5 allocs/op
BenchmarkLARS_GPlus2Params               5000000               238 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GPlus2Params             500000              4016 ns/op            1056 B/op         10 allocs/op
BenchmarkMartini_GPlus2Params             100000             21253 ns/op            1200 B/op         13 allocs/op
BenchmarkPat_GPlus2Params                 200000              8632 ns/op            2256 B/op         34 allocs/op
BenchmarkPossum_GPlus2Params             1000000              2171 ns/op             560 B/op          6 allocs/op
BenchmarkR2router_GPlus2Params           1000000              1340 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_GPlus2Params              3000000               557 ns/op              96 B/op          1 allocs/op
BenchmarkTango_GPlus2Params              1000000              2186 ns/op             344 B/op          8 allocs/op
BenchmarkTigerTonic_GPlus2Params          200000              9060 ns/op            1488 B/op         24 allocs/op
BenchmarkTraffic_GPlus2Params             100000             20324 ns/op            3272 B/op         31 allocs/op
BenchmarkVulcan_GPlus2Params             1000000              2039 ns/op              98 B/op          3 allocs/op
BenchmarkAce_GPlusAll                     300000              6603 ns/op             640 B/op         11 allocs/op
BenchmarkBear_GPlusAll                    100000             22363 ns/op            5488 B/op         61 allocs/op
BenchmarkBeego_GPlusAll                    50000             38757 ns/op            4784 B/op         52 allocs/op
BenchmarkBone_GPlusAll                     20000             54916 ns/op           10336 B/op         98 allocs/op
BenchmarkDenco_GPlusAll                   300000              4959 ns/op             672 B/op         11 allocs/op
BenchmarkEcho_GPlusAll                    200000              6558 ns/op             416 B/op         13 allocs/op
BenchmarkGin_GPlusAll                     500000              2757 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_GPlusAll               50000             34615 ns/op            8040 B/op        103 allocs/op
BenchmarkGoji_GPlusAll                    100000             16002 ns/op            3696 B/op         22 allocs/op
BenchmarkGojiv2_GPlusAll                   50000             35060 ns/op           12624 B/op        115 allocs/op
BenchmarkGoJsonRest_GPlusAll               50000             41479 ns/op            8117 B/op        170 allocs/op
BenchmarkGoRestful_GPlusAll                10000            131653 ns/op           32024 B/op        275 allocs/op
BenchmarkGorillaMux_GPlusAll               10000            101380 ns/op           13296 B/op        142 allocs/op
BenchmarkHttpRouter_GPlusAll              500000              3711 ns/op             640 B/op         11 allocs/op
BenchmarkHttpTreeMux_GPlusAll             100000             14438 ns/op            4032 B/op         38 allocs/op
BenchmarkKocha_GPlusAll                   200000              8039 ns/op             976 B/op         43 allocs/op
BenchmarkLARS_GPlusAll                    500000              2630 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_GPlusAll                  30000             51123 ns/op           13152 B/op        128 allocs/op
BenchmarkMartini_GPlusAll                  10000            176157 ns/op           14016 B/op        145 allocs/op
BenchmarkPat_GPlusAll                      20000             69911 ns/op           16576 B/op        298 allocs/op
BenchmarkPossum_GPlusAll                  100000             20716 ns/op            5408 B/op         39 allocs/op
BenchmarkR2router_GPlusAll                100000             17463 ns/op            5040 B/op         63 allocs/op
BenchmarkRivet_GPlusAll                   300000              5142 ns/op             768 B/op         11 allocs/op
BenchmarkTango_GPlusAll                    50000             27321 ns/op            3656 B/op        104 allocs/op
BenchmarkTigerTonic_GPlusAll               20000             77597 ns/op           14512 B/op        288 allocs/op
BenchmarkTraffic_GPlusAll                  10000            151406 ns/op           37360 B/op        392 allocs/op
BenchmarkVulcan_GPlusAll                  100000             18555 ns/op            1274 B/op         39 allocs/op
>>>>>>> cbc9bb05... fixup add vendor back
```

## Parse.com

```
<<<<<<< HEAD
BenchmarkGin_ParseStatic                 8683893               140 ns/op               0 B/op          0 allocs/op

BenchmarkAce_ParseStatic                 7255582               160 ns/op               0 B/op          0 allocs/op
BenchmarkAero_ParseStatic               11960128                95.0 ns/op             0 B/op          0 allocs/op
BenchmarkBear_ParseStatic                1791033               659 ns/op             120 B/op          3 allocs/op
BenchmarkBeego_ParseStatic                937918              1688 ns/op             352 B/op          3 allocs/op
BenchmarkBone_ParseStatic                1261682               949 ns/op             144 B/op          3 allocs/op
BenchmarkChi_ParseStatic                 1000000              1303 ns/op             432 B/op          3 allocs/op
BenchmarkDenco_ParseStatic              23731242                49.8 ns/op             0 B/op          0 allocs/op
BenchmarkEcho_ParseStatic               10585060               116 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_ParseStatic          1000000              1156 ns/op             296 B/op          5 allocs/op
BenchmarkGoji_ParseStatic                3927530               300 ns/op               0 B/op          0 allocs/op
BenchmarkGojiv2_ParseStatic               474836              3281 ns/op            1312 B/op         10 allocs/op
BenchmarkGoJsonRest_ParseStatic          1000000              1445 ns/op             329 B/op         11 allocs/op
BenchmarkGoRestful_ParseStatic            101262             11612 ns/op            4256 B/op         13 allocs/op
BenchmarkGorillaMux_ParseStatic           562705              3530 ns/op             976 B/op          9 allocs/op
BenchmarkGowwwRouter_ParseStatic        16479007                69.5 ns/op             0 B/op          0 allocs/op
BenchmarkHttpRouter_ParseStatic         23205590                51.5 ns/op             0 B/op          0 allocs/op
BenchmarkHttpTreeMux_ParseStatic        10763127               106 ns/op               0 B/op          0 allocs/op
BenchmarkKocha_ParseStatic              17850259                60.9 ns/op             0 B/op          0 allocs/op
BenchmarkLARS_ParseStatic               10727432               108 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_ParseStatic              685586              2665 ns/op             736 B/op          8 allocs/op
BenchmarkMartini_ParseStatic              200642              7158 ns/op             768 B/op          9 allocs/op
BenchmarkPat_ParseStatic                 1000000              1139 ns/op             240 B/op          5 allocs/op
BenchmarkPossum_ParseStatic              1000000              1241 ns/op             416 B/op          3 allocs/op
BenchmarkR2router_ParseStatic            2035426               597 ns/op             144 B/op          4 allocs/op
BenchmarkRivet_ParseStatic               9707011               127 ns/op               0 B/op          0 allocs/op
BenchmarkTango_ParseStatic                910617              1693 ns/op             248 B/op          8 allocs/op
BenchmarkTigerTonic_ParseStatic          3168885               385 ns/op              48 B/op          1 allocs/op
BenchmarkTraffic_ParseStatic              493339              4264 ns/op            1256 B/op         19 allocs/op
BenchmarkVulcan_ParseStatic              1394142               848 ns/op              98 B/op          3 allocs/op

BenchmarkAce_ParseParam                  3106903               387 ns/op              64 B/op          1 allocs/op
BenchmarkAero_ParseParam                 8045266               141 ns/op               0 B/op          0 allocs/op
BenchmarkBear_ParseParam                 1000000              1434 ns/op             467 B/op          5 allocs/op
BenchmarkBeego_ParseParam                 951460              1937 ns/op             352 B/op          3 allocs/op
BenchmarkBone_ParseParam                  855555              2776 ns/op             896 B/op          7 allocs/op
BenchmarkChi_ParseParam                  1000000              1457 ns/op             432 B/op          3 allocs/op
BenchmarkDenco_ParseParam                4084116               301 ns/op              64 B/op          1 allocs/op
BenchmarkEcho_ParseParam                 8440170               142 ns/op               0 B/op          0 allocs/op
BenchmarkGin_ParseParam                  7716948               157 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_ParseParam            886284              2045 ns/op             664 B/op          8 allocs/op
BenchmarkGoji_ParseParam                 1000000              1167 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_ParseParam                269731              3945 ns/op            1360 B/op         12 allocs/op
BenchmarkGoJsonRest_ParseParam            719587              2277 ns/op             649 B/op         13 allocs/op
BenchmarkGoRestful_ParseParam              96408             11925 ns/op            4576 B/op         14 allocs/op
BenchmarkGorillaMux_ParseParam            289303              4154 ns/op            1280 B/op         10 allocs/op
BenchmarkGowwwRouter_ParseParam          1000000              1070 ns/op             432 B/op          3 allocs/op
BenchmarkHttpRouter_ParseParam           4917758               232 ns/op              64 B/op          1 allocs/op
BenchmarkHttpTreeMux_ParseParam          1445443               828 ns/op             352 B/op          3 allocs/op
BenchmarkKocha_ParseParam                3116233               382 ns/op              56 B/op          3 allocs/op
BenchmarkLARS_ParseParam                10584750               113 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_ParseParam               413617              3872 ns/op            1072 B/op         10 allocs/op
BenchmarkMartini_ParseParam               166545              7605 ns/op            1072 B/op         10 allocs/op
BenchmarkPat_ParseParam                   491829              3394 ns/op             992 B/op         15 allocs/op
BenchmarkPossum_ParseParam               1000000              1692 ns/op             496 B/op          5 allocs/op
BenchmarkR2router_ParseParam             1000000              1059 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_ParseParam                3572359               311 ns/op              48 B/op          1 allocs/op
BenchmarkTango_ParseParam                 787552              1889 ns/op             280 B/op          8 allocs/op
BenchmarkTigerTonic_ParseParam            487208              3706 ns/op             784 B/op         15 allocs/op
BenchmarkTraffic_ParseParam               186190              5812 ns/op            1896 B/op         21 allocs/op
BenchmarkVulcan_ParseParam               1275432               892 ns/op              98 B/op          3 allocs/op

BenchmarkAce_Parse2Params                2959621               412 ns/op              64 B/op          1 allocs/op
BenchmarkAero_Parse2Params               6208641               192 ns/op               0 B/op          0 allocs/op
BenchmarkBear_Parse2Params               1000000              1512 ns/op             496 B/op          5 allocs/op
BenchmarkBeego_Parse2Params               761940              1973 ns/op             352 B/op          3 allocs/op
BenchmarkBone_Parse2Params                715987              2582 ns/op             848 B/op          6 allocs/op
BenchmarkChi_Parse2Params                1000000              1495 ns/op             432 B/op          3 allocs/op
BenchmarkDenco_Parse2Params              3585452               341 ns/op              64 B/op          1 allocs/op
BenchmarkEcho_Parse2Params               5193693               204 ns/op               0 B/op          0 allocs/op
BenchmarkGin_Parse2Params                5338316               236 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_Parse2Params          939637              2299 ns/op             712 B/op          9 allocs/op
BenchmarkGoji_Parse2Params               1000000              1094 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_Parse2Params              339514              3733 ns/op            1344 B/op         11 allocs/op
BenchmarkGoJsonRest_Parse2Params          512572              2733 ns/op             713 B/op         14 allocs/op
BenchmarkGoRestful_Parse2Params            95913             12973 ns/op            4928 B/op         14 allocs/op
BenchmarkGorillaMux_Parse2Params          261208              4758 ns/op            1296 B/op         10 allocs/op
BenchmarkGowwwRouter_Parse2Params        1000000              1084 ns/op             432 B/op          3 allocs/op
BenchmarkHttpRouter_Parse2Params         4399953               277 ns/op              64 B/op          1 allocs/op
BenchmarkHttpTreeMux_Parse2Params        1000000              1198 ns/op             384 B/op          4 allocs/op
BenchmarkKocha_Parse2Params              1669431               683 ns/op             128 B/op          5 allocs/op
BenchmarkLARS_Parse2Params               8535754               142 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_Parse2Params             424590              3959 ns/op            1072 B/op         10 allocs/op
BenchmarkMartini_Parse2Params             162448              8141 ns/op            1152 B/op         11 allocs/op
BenchmarkPat_Parse2Params                 431336              3484 ns/op             752 B/op         16 allocs/op
BenchmarkPossum_Parse2Params             1000000              1721 ns/op             496 B/op          5 allocs/op
BenchmarkR2router_Parse2Params           1000000              1136 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_Parse2Params              2630935               442 ns/op              96 B/op          1 allocs/op
BenchmarkTango_Parse2Params               759218              1876 ns/op             312 B/op          8 allocs/op
BenchmarkTigerTonic_Parse2Params          290810              5558 ns/op            1168 B/op         22 allocs/op
BenchmarkTraffic_Parse2Params             181099              6917 ns/op            1944 B/op         22 allocs/op
BenchmarkVulcan_Parse2Params             1000000              1080 ns/op              98 B/op          3 allocs/op

BenchmarkAce_ParseAll                     162906              7888 ns/op             640 B/op         16 allocs/op
BenchmarkAero_ParseAll                    219260              4833 ns/op               0 B/op          0 allocs/op
BenchmarkBear_ParseAll                     37566             32863 ns/op            8928 B/op        110 allocs/op
BenchmarkBeego_ParseAll                    25400             46518 ns/op            9152 B/op         78 allocs/op
BenchmarkBone_ParseAll                     19568             61814 ns/op           16208 B/op        147 allocs/op
BenchmarkChi_ParseAll                      30562             38281 ns/op           11232 B/op         78 allocs/op
BenchmarkDenco_ParseAll                   232554              6371 ns/op             928 B/op         16 allocs/op
BenchmarkEcho_ParseAll                    224400              5090 ns/op               0 B/op          0 allocs/op
BenchmarkGin_ParseAll                     189829              6134 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_ParseAll               25446             47000 ns/op           13728 B/op        181 allocs/op
BenchmarkGoji_ParseAll                     50503             22949 ns/op            5376 B/op         32 allocs/op
BenchmarkGojiv2_ParseAll                   12806             93106 ns/op           34448 B/op        277 allocs/op
BenchmarkGoJsonRest_ParseAll               20764             57021 ns/op           13866 B/op        321 allocs/op
BenchmarkGoRestful_ParseAll                 4234            317238 ns/op          117600 B/op        354 allocs/op
BenchmarkGorillaMux_ParseAll               10000            146942 ns/op           30288 B/op        250 allocs/op
BenchmarkGowwwRouter_ParseAll              62548             19363 ns/op            6912 B/op         48 allocs/op
BenchmarkHttpRouter_ParseAll              286662              5091 ns/op             640 B/op         16 allocs/op
BenchmarkHttpTreeMux_ParseAll              66952             18262 ns/op            5728 B/op         51 allocs/op
BenchmarkKocha_ParseAll                   109771              9811 ns/op            1112 B/op         54 allocs/op
BenchmarkLARS_ParseAll                    272516              3976 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_ParseAll                  17094             71634 ns/op           19136 B/op        208 allocs/op
BenchmarkMartini_ParseAll                   6799            208122 ns/op           25072 B/op        253 allocs/op
BenchmarkPat_ParseAll                      15993             74594 ns/op           15216 B/op        308 allocs/op
BenchmarkPossum_ParseAll                   34897             33398 ns/op           10816 B/op         78 allocs/op
BenchmarkR2router_ParseAll                 46909             25410 ns/op            8352 B/op        120 allocs/op
BenchmarkRivet_ParseAll                   185193              7725 ns/op             912 B/op         16 allocs/op
BenchmarkTango_ParseAll                    24481             47963 ns/op            7168 B/op        208 allocs/op
BenchmarkTigerTonic_ParseAll               15236             79623 ns/op           16048 B/op        332 allocs/op
BenchmarkTraffic_ParseAll                   8955            169411 ns/op           45520 B/op        605 allocs/op
BenchmarkVulcan_ParseAll                   40406             28971 ns/op            2548 B/op         78 allocs/op
=======
BenchmarkGin_ParseStatic                10000000               133 ns/op               0 B/op          0 allocs/op

BenchmarkAce_ParseStatic                 5000000               241 ns/op               0 B/op          0 allocs/op
BenchmarkBear_ParseStatic                2000000               728 ns/op             120 B/op          3 allocs/op
BenchmarkBeego_ParseStatic               1000000              2623 ns/op             368 B/op          4 allocs/op
BenchmarkBone_ParseStatic                1000000              1285 ns/op             144 B/op          3 allocs/op
BenchmarkDenco_ParseStatic              30000000                57.8 ns/op             0 B/op          0 allocs/op
BenchmarkEcho_ParseStatic                5000000               342 ns/op              32 B/op          1 allocs/op
BenchmarkGocraftWeb_ParseStatic          1000000              1478 ns/op             296 B/op          5 allocs/op
BenchmarkGoji_ParseStatic                3000000               415 ns/op               0 B/op          0 allocs/op
BenchmarkGojiv2_ParseStatic              1000000              2087 ns/op             928 B/op          7 allocs/op
BenchmarkGoJsonRest_ParseStatic          1000000              1712 ns/op             329 B/op         11 allocs/op
BenchmarkGoRestful_ParseStatic            200000             11072 ns/op            3224 B/op         22 allocs/op
BenchmarkGorillaMux_ParseStatic           500000              4129 ns/op             752 B/op         11 allocs/op
BenchmarkHttpRouter_ParseStatic         30000000                52.4 ns/op             0 B/op          0 allocs/op
BenchmarkHttpTreeMux_ParseStatic        20000000               109 ns/op               0 B/op          0 allocs/op
BenchmarkKocha_ParseStatic              20000000                81.8 ns/op             0 B/op          0 allocs/op
BenchmarkLARS_ParseStatic               10000000               150 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_ParseStatic             1000000              3288 ns/op             768 B/op          9 allocs/op
BenchmarkMartini_ParseStatic              200000              9110 ns/op             768 B/op          9 allocs/op
BenchmarkPat_ParseStatic                 1000000              1135 ns/op             240 B/op          5 allocs/op
BenchmarkPossum_ParseStatic              1000000              1557 ns/op             416 B/op          3 allocs/op
BenchmarkR2router_ParseStatic            2000000               730 ns/op             144 B/op          4 allocs/op
BenchmarkRivet_ParseStatic              10000000               121 ns/op               0 B/op          0 allocs/op
BenchmarkTango_ParseStatic               1000000              1688 ns/op             248 B/op          8 allocs/op
BenchmarkTigerTonic_ParseStatic          3000000               427 ns/op              48 B/op          1 allocs/op
BenchmarkTraffic_ParseStatic              500000              5962 ns/op            1816 B/op         20 allocs/op
BenchmarkVulcan_ParseStatic              2000000               969 ns/op              98 B/op          3 allocs/op
BenchmarkAce_ParseParam                  3000000               497 ns/op              64 B/op          1 allocs/op
BenchmarkBear_ParseParam                 1000000              1473 ns/op             467 B/op          5 allocs/op
BenchmarkBeego_ParseParam                1000000              2384 ns/op             368 B/op          4 allocs/op
BenchmarkBone_ParseParam                 1000000              2513 ns/op             768 B/op          6 allocs/op
BenchmarkDenco_ParseParam                5000000               364 ns/op              64 B/op          1 allocs/op
BenchmarkEcho_ParseParam                 5000000               418 ns/op              32 B/op          1 allocs/op
BenchmarkGin_ParseParam                 10000000               163 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_ParseParam           1000000              2361 ns/op             664 B/op          8 allocs/op
BenchmarkGoji_ParseParam                 1000000              1590 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_ParseParam               1000000              2851 ns/op             976 B/op          9 allocs/op
BenchmarkGoJsonRest_ParseParam           1000000              2965 ns/op             649 B/op         13 allocs/op
BenchmarkGoRestful_ParseParam             200000             12207 ns/op            3544 B/op         23 allocs/op
BenchmarkGorillaMux_ParseParam            500000              5187 ns/op            1088 B/op         12 allocs/op
BenchmarkHttpRouter_ParseParam           5000000               275 ns/op              64 B/op          1 allocs/op
BenchmarkHttpTreeMux_ParseParam          1000000              1108 ns/op             352 B/op          3 allocs/op
BenchmarkKocha_ParseParam                3000000               495 ns/op              56 B/op          3 allocs/op
BenchmarkLARS_ParseParam                10000000               192 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_ParseParam               500000              4103 ns/op            1056 B/op         10 allocs/op
BenchmarkMartini_ParseParam               200000              9878 ns/op            1072 B/op         10 allocs/op
BenchmarkPat_ParseParam                   500000              3657 ns/op            1120 B/op         17 allocs/op
BenchmarkPossum_ParseParam               1000000              2084 ns/op             560 B/op          6 allocs/op
BenchmarkR2router_ParseParam             1000000              1251 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_ParseParam                5000000               335 ns/op              48 B/op          1 allocs/op
BenchmarkTango_ParseParam                1000000              1854 ns/op             280 B/op          8 allocs/op
BenchmarkTigerTonic_ParseParam            500000              4582 ns/op            1008 B/op         17 allocs/op
BenchmarkTraffic_ParseParam               200000              8125 ns/op            2248 B/op         23 allocs/op
BenchmarkVulcan_ParseParam               1000000              1148 ns/op              98 B/op          3 allocs/op
BenchmarkAce_Parse2Params                3000000               539 ns/op              64 B/op          1 allocs/op
BenchmarkBear_Parse2Params               1000000              1778 ns/op             496 B/op          5 allocs/op
BenchmarkBeego_Parse2Params              1000000              2519 ns/op             368 B/op          4 allocs/op
BenchmarkBone_Parse2Params               1000000              2596 ns/op             720 B/op          5 allocs/op
BenchmarkDenco_Parse2Params              3000000               492 ns/op              64 B/op          1 allocs/op
BenchmarkEcho_Parse2Params               3000000               484 ns/op              32 B/op          1 allocs/op
BenchmarkGin_Parse2Params               10000000               193 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_Parse2Params         1000000              2575 ns/op             712 B/op          9 allocs/op
BenchmarkGoji_Parse2Params               1000000              1373 ns/op             336 B/op          2 allocs/op
BenchmarkGojiv2_Parse2Params              500000              2416 ns/op             960 B/op          8 allocs/op
BenchmarkGoJsonRest_Parse2Params          300000              3452 ns/op             713 B/op         14 allocs/op
BenchmarkGoRestful_Parse2Params           100000             17719 ns/op            6008 B/op         25 allocs/op
BenchmarkGorillaMux_Parse2Params          300000              5102 ns/op            1088 B/op         11 allocs/op
BenchmarkHttpRouter_Parse2Params         5000000               303 ns/op              64 B/op          1 allocs/op
BenchmarkHttpTreeMux_Parse2Params        1000000              1372 ns/op             384 B/op          4 allocs/op
BenchmarkKocha_Parse2Params              2000000               874 ns/op             128 B/op          5 allocs/op
BenchmarkLARS_Parse2Params              10000000               192 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_Parse2Params             500000              3871 ns/op            1056 B/op         10 allocs/op
BenchmarkMartini_Parse2Params             200000              9954 ns/op            1152 B/op         11 allocs/op
BenchmarkPat_Parse2Params                 500000              4194 ns/op             832 B/op         17 allocs/op
BenchmarkPossum_Parse2Params             1000000              2121 ns/op             560 B/op          6 allocs/op
BenchmarkR2router_Parse2Params           1000000              1415 ns/op             432 B/op          5 allocs/op
BenchmarkRivet_Parse2Params              3000000               457 ns/op              96 B/op          1 allocs/op
BenchmarkTango_Parse2Params              1000000              1914 ns/op             312 B/op          8 allocs/op
BenchmarkTigerTonic_Parse2Params          300000              6895 ns/op            1408 B/op         24 allocs/op
BenchmarkTraffic_Parse2Params             200000              8317 ns/op            2040 B/op         22 allocs/op
BenchmarkVulcan_Parse2Params             1000000              1274 ns/op              98 B/op          3 allocs/op
BenchmarkAce_ParseAll                     200000             10401 ns/op             640 B/op         16 allocs/op
BenchmarkBear_ParseAll                     50000             37743 ns/op            8928 B/op        110 allocs/op
BenchmarkBeego_ParseAll                    20000             63193 ns/op            9568 B/op        104 allocs/op
BenchmarkBone_ParseAll                     20000             61767 ns/op           14160 B/op        131 allocs/op
BenchmarkDenco_ParseAll                   300000              7036 ns/op             928 B/op         16 allocs/op
BenchmarkEcho_ParseAll                    200000             11824 ns/op             832 B/op         26 allocs/op
BenchmarkGin_ParseAll                     300000              4199 ns/op               0 B/op          0 allocs/op
BenchmarkGocraftWeb_ParseAll               30000             51758 ns/op           13728 B/op        181 allocs/op
BenchmarkGoji_ParseAll                     50000             29614 ns/op            5376 B/op         32 allocs/op
BenchmarkGojiv2_ParseAll                   20000             68676 ns/op           24464 B/op        199 allocs/op
BenchmarkGoJsonRest_ParseAll               20000             76135 ns/op           13866 B/op        321 allocs/op
BenchmarkGoRestful_ParseAll                 5000            389487 ns/op          110928 B/op        600 allocs/op
BenchmarkGorillaMux_ParseAll               10000            221250 ns/op           24864 B/op        292 allocs/op
BenchmarkHttpRouter_ParseAll              200000              6444 ns/op             640 B/op         16 allocs/op
BenchmarkHttpTreeMux_ParseAll              50000             30702 ns/op            5728 B/op         51 allocs/op
BenchmarkKocha_ParseAll                   200000             13712 ns/op            1112 B/op         54 allocs/op
BenchmarkLARS_ParseAll                    300000              6925 ns/op               0 B/op          0 allocs/op
BenchmarkMacaron_ParseAll                  20000             96278 ns/op           24576 B/op        250 allocs/op
BenchmarkMartini_ParseAll                   5000            271352 ns/op           25072 B/op        253 allocs/op
BenchmarkPat_ParseAll                      20000             74941 ns/op           17264 B/op        343 allocs/op
BenchmarkPossum_ParseAll                   50000             39947 ns/op           10816 B/op         78 allocs/op
BenchmarkR2router_ParseAll                 50000             42479 ns/op            8352 B/op        120 allocs/op
BenchmarkRivet_ParseAll                   200000              7726 ns/op             912 B/op         16 allocs/op
BenchmarkTango_ParseAll                    30000             50014 ns/op            7168 B/op        208 allocs/op
BenchmarkTigerTonic_ParseAll               10000            106550 ns/op           19728 B/op        379 allocs/op
BenchmarkTraffic_ParseAll                  10000            216037 ns/op           57776 B/op        642 allocs/op
BenchmarkVulcan_ParseAll                   50000             34379 ns/op            2548 B/op         78 allocs/op
>>>>>>> cbc9bb05... fixup add vendor back
```
