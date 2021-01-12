#!/bin/bash

if [ $# -ne 2 ] ; then
	echo 1>&2 Usage: $0 PlugNumber '<on>|<off>'
	exit 1
fi

PlugNumber=`expr "$1" \* 1 2>/dev/null`
if [ $? -ne 0 ] ; then
	echo 1>&2 First parameter must be an integer
	exit 1
fi

if [ $PlugNumber -lt 0 -o $PlugNumber -gt 9 ] ; then
	echo 1>&2 "First parameter must be an integer in the range [0-9]"
	exit 1
fi

Command=$2

On=`case $Command in
	"on"|"ON"|"On"|"true"|"True"|"TRUE")
		echo true
		;;
	"off"|"OFF"|"Off"|"false"|"False"|"FALSE")
		echo false
		;;
	*)
		echo 1>&2 "Second parameter must be \"on\" or \"off\""
		echo quit
		;;
esac`

if [ $On == "quit" ] ; then
	exit 1
fi

#echo PlugNumber is $PlugNumber
#echo On is $On

echo mosquitto_pub -t devices/tp-plug-0${PlugNumber}/outlet/on/set -m ${On}
mosquitto_pub -t devices/tp-plug-0${PlugNumber}/outlet/on/set -m ${On}
