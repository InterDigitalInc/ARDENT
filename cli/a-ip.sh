#!/bin/bash
#
# Author: Sebastian Robitzsch <sebastian.robitzsch@interdigital.com>
#
# Implementation of the A_{IP} RESTful ARDENT API

aIpHelp()
{
	RES=$1

	if [ $# -eq 0 ]
	then
		echo -e "\tardent descr"
		echo -e "\tardent rc"
	elif [ $# -eq 1 ]
	then
		case "$RES" in
			"descr")
				echo -e "\tardent descr add <DESCRIPTOR>"
				echo -e "\tardent descr delete"
				;;
			"rc")
				echo -e "\tardent rc admin add <RUNCOM>"
				echo -e "\tardent rc admin delete"
				echo -e "\tardent rc tenant add <RUNCOM>"
				echo -e "\tardent rc tenant delete"
				;;
			*)
				echo "Unknown argument '$1'"
				;;
		esac
	else
		if [ $RES == "descr" ]
		then
			if [ $2 == "add" ]
			then
				echo -e "\tardent descr add <DESCRIPTOR>"
			fi
		elif [ $RES == "rc" ]
		then
			if [ $2 == "admin" ]
			then
				echo -e "\tardent rc admin add <RUNCOM>"
				echo -e "\tardent rc admin delete"
			elif [ $2 == "tenant" ]
			then
				echo -e "\tardent rc tenant add <RUNCOM>"
				echo -e "\tardent rc tenant delete"
			else
				echo "Unknown argument '$2'"
			fi
		fi
	fi
}

aIpDescr()
{
	case "$1" in
	"add")
		if [ $# -ne 2 ]
		then
			aIpHelp descr
			exit 1
		fi

		echo -ne "---\nAdding new descriptor\n---\n"
		descrAdd $2
		;;
	"delete")
		echo -ne "---\nDeleting descriptor\n---\n"
		descrDelete
		;;
	*)
		echo -ne "---\nUnknown argument '$1'\n---\n"
		;;
	esac
}

aIpRc()
{
	TYPE=$1
	ACTION=$2
	RC=$3

        case "$ACTION" in
        "add")
		if [ $# -lt 3 ]
		then
			aIpHelp rc $TYPE $ACTION
			exit 1
		fi

		echo -ne "---\nAdding new $TYPE runcom\n---\n"
                rcAdd $TYPE $RC
                ;;
        "delete")
		echo -ne "---\nDeleting $TYPE runcom\n---\n"
                rcDelete $TYPE
                ;;
        *)
                echo -ne "---\nUnknown argument '$ACTION'\n---\n"
                ;;
        esac
}

###############################################################################

descrAdd()
{
	DESCR=$1

	http PUT http://$ARDENT:$ARDENT_PORT/infra/descriptor @$DESCR
}

descrDelete()
{
	http DELETE http://$ARDENT:$ARDENT_PORT/infra/descriptor
}

rcAdd()
{
	TYPE=$1
	RC=$2

	http PUT http://$ARDENT:$ARDENT_PORT/infra/rc/$TYPE @$RC
}

rcDelete()
{
	TYPE=$1

	http DELETE http://$ARDENT:$ARDENT_PORT/infra/rc/$TYPE
}

