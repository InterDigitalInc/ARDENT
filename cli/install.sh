#!/bin/bash
#
#
#
# Installing ARDENT CLI to /usr/local/src and set symbolic link in /usr/local/bin

cd `dirname $0`

ARDENT_DIR=/usr/local/src/ardent

echo "Creating $ARDENT_DIR"
mkdir -p $ARDENT_DIR
echo "Copying files"
cp -rfv ardent.sh $ARDENT_DIR/
cp -v *.sh $ARDENT_DIR/

if [ ! -L /usr/local/bin/ardent ]
then
	echo "Creating symbolic links"
	ln -s /usr/local/src/ardent/ardent.sh /usr/local/bin/ardent
fi

echo "$0 done"
