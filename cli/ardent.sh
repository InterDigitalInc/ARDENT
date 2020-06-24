#!/bin/bash
#
# Author: Sebastian Robitzsch <sebastian.robitzsch@interdigital.com>

# check
if ! which http >/dev/null
then
	echo "Installing httpie packet first"
	if ! apt-get install httpie -y
	then
		echo "failed"
		exit 1
	fi
fi

if [[ ! -v ARDENT ]]
then
	echo "runcom has not been sourced"
	exit 1
fi

PATH_INSTALL=/usr/local/src/ardent

# API implementations
source $PATH_INSTALL/a-ip.sh
source $PATH_INSTALL/a-h.sh
source $PATH_INSTALL/a-s.sh
source $PATH_INSTALL/a-sc.sh

if [ $# -eq 0 ]
then
	aIpHelp
	aHHelp
	aSHelp
	aScHelp
	exit 0
fi


case "$1" in
"descr")
	if [ $# -lt 2 ]
	then
		aIpHelp $1
		exit 1
	fi

	aIpDescr $2 $3
	;;
"rc")
	if [ $# -le 2 ]
	then
		aIpHelp $1 $2
		exit 1
	fi

	if [ $# -ge 3 ]
	then
		aIpRc $2 $3 $4
	fi
	;;
"hot")
	if [ $# -ne 2 ]
	then
		aHHelp $1
		exit 1
	fi

	aHHot $2
	;;
"stack")
	if [ $# -lt 2 ]
	then
		aSHelp $1
		exit 1
	fi

	aSStack $2 $3
	;;
"check")
	if [ $# -lt 2 ]
	then
		aScHelp $1
		exit 1
	fi

	aScCheck $2 $3
	;;
*)
	echo "unknown argument '$1'"
	;;
esac
