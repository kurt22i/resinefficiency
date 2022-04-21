#NoEnv  ; Recommended for performance and compatibility with future AutoHotkey releases.
#Warn  ; Enable warnings to assist with detecting common errors.
SendMode Input  ; Recommended for new scripts due to its superior speed and reliability.
SetWorkingDir %A_ScriptDir%  ; Ensures a consistent starting directory.

a & s::

Domains := [0]
CharLocs := [0,0,0,0]

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
		Sleep, 100
		Click, 970 700
		Sleep, 100
		Send, ^a
		Sleep, 100
		Send, ^c
		Sleep, 100
		Click, 1738 525
		Sleep, 100
		;Click, 1445 1059
		Click, 825 13
		Sleep, 100
		Click, 1601 465
		Sleep, 100
		Click, 933 713
		Sleep, 100
		Send, ^v
		Sleep, 100
		Click, 1021 463
		Sleep, 100
		Send, {End}
		Sleep, 400
		Click, 323 833
		Sleep, 100
		Click, 535 101
		Sleep, 100
		Click, 1000 200
		Sleep, 100
		Send, {End}
		Sleep, 800
		Send, {End}
		Sleep, 100
		Click, 277 744
		Sleep, 3000
		Send, {End}
		Sleep, 500
		Click, 1639 299
		Sleep, 100
		;Click, 1057 155
		Send, {Enter}
		Sleep, 1000
		Send, {Home}
		Sleep, 800
		Click, 685 97
		Sleep, 100
		Click, 467 465
		Sleep, 100
		;Click, 1135 159
		Send, {Enter}
		Sleep, 100
		Run %fileloc%
		Sleep, 100
		Click, 970 700
		Sleep, 100
		Send, ^a
		Sleep, 100
		Send, ^v
		Sleep, 100
		Send, ^s
		Sleep, 100
		Click, 1738 525
		Sleep, 100
		count++
	}
}
return

q & w::
ExitApp
return