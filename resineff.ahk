#NoEnv  ; Recommended for performance and compatibility with future AutoHotkey releases.
#Warn  ; Enable warnings to assist with detecting common errors.
SendMode Input  ; Recommended for new scripts due to its superior speed and reliability.
SetWorkingDir %A_ScriptDir%  ; Ensures a consistent starting directory.

a & s::

Domains := [0]
CharLocs := [621,515,0,0]

CoordMode, Mouse, Screen

for index, element in Domains
{
	count := 0
	Loop, 79
	{
		str := "" count
		fileloc := element "gojson" str ".txt"
		;fileloc := "\" element "\gojson" %open% ".txt"
		;filename := "gojson" %open% ".txt"
		;fileloc2 := "\" element
		Run %fileloc%
		Sleep, 200
		Click, 970 700
		Sleep, 200
		Send, ^a
		Sleep, 200
		Send, ^c
		Sleep, 200
		Click, 1738 525
		Sleep, 200
		;Click, 1445 1059
		Click, 825 13
		Sleep, 200
		Click, 1601 465
		Sleep, 200
		Click, 933 713
		Sleep, 200
		Send, ^v
		Sleep, 200
		Click, 1021 463
		Sleep, 200
		Send, {End}
		Sleep, 400
		Click, 323 833
		Sleep, 200
		Click, 535 101
		Sleep, 1100
		count2 := 0
		;Loop, 2 {
			Click, 525 272
			Sleep, 400
			;Click, 300 CharLocs[count2]
			Click 300, 621
			Sleep, 200
			Send, {End}
			Sleep, 1000
			Send, {End}
			Sleep, 300
			Click, 277 744
			Sleep, 5000
			Send, {End}
			Sleep, 700
			Click, 1639 321
			Sleep, 200
			Send, {Enter}
			Sleep, 2000
			Send, {Home}
			Sleep, 800
			count2++
		;}
		Click, 525 272
			Sleep, 400
			;Click, 300 CharLocs[count2]
			Click 300, 515
			Sleep, 200
			Send, {End}
			Sleep, 1100
			Send, {End}
			Sleep, 400
			Click, 277 744
			Sleep, 5000
			Send, {End}
			Sleep, 700
			Click, 1639 299
			Sleep, 200
			Send, {Enter}
			Sleep, 2000
			Send, {Home}
			Sleep, 800
		
		Click, 685 97
		Sleep, 200
		Click, 467 465
		Sleep, 200
		;Click, 1135 159
		Send, {Enter}
		Sleep, 200
		Run %fileloc%
		Sleep, 200
		Click, 970 700
		Sleep, 200
		Send, ^a
		Sleep, 200
		Send, ^v
		Sleep, 200
		Send, ^s
		Sleep, 200
		Click, 1738 525
		Sleep, 200
		count++
	}
}
return

q & w::
ExitApp
return