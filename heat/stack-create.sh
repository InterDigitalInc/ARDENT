#!/bin/bash
set -euo pipefail
cd `dirname $0`

if [ $# != 1 ] ; then
    echo "usage: $0 <i2cat|ide|soton|uob>"
    exit 1
fi

STACK=flame-platform
TEMPLATE=$1/stack-$STACK.yaml
BASE_VARS=$FLAME_ROOT/base-vars.sh
PLATFORM_VARS=$1/$1"-vars.sh"

if [ ! -f $TEMPLATE ]
then
    echo "$TEMPLATE does not exist"
    exit 1
fi

if [ ! -f $BASE_VARS ]
then
    echo "$BASE_VARS does not exist"
    exit 1
fi

if [ ! -f $PLATFORM_VARS ]
then
    echo "$PLATFORM_VARS does not exist"
    exit 1
fi

source $BASE_VARS
source $PLATFORM_VARS

parms=
in_parms=0

while read -r line ; do
    if [ $in_parms -eq 1 ] ; then
        if echo $line | grep -q resources: ; then
            in_parms=0
        elif echo $line | grep -q :\s*$ ; then
            parm=`echo $line | sed -e "s/\s//g" | sed -e "s/://g"`
            var=`echo $parm | tr [a-z] [A-Z] | sed -e "s/-/_/g"`
            eval "val=\$$var"
            parms="${parms} --parameter $parm=$val"
        fi
    elif  echo $line | grep -q parameters: ; then
        in_parms=1
    fi
done < $TEMPLATE

openstack stack create $parms -t $TEMPLATE $STACK

echo -ne "wait for stack create"

while openstack stack show $STACK | grep CREATE_IN_PROGRESS >/dev/null ; do
    echo -n "."
    sleep 10
done

echo -ne "\n"
openstack stack show $STACK -f json | jq '.stack_status'

if ! openstack stack show $STACK -f json | jq '.stack_status' | grep COMPLETE > /dev/null
then
	openstack stack show $STACK -f json | jq '.stack_status_reason'
fi
	
