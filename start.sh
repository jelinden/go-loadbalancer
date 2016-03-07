#!/bin/bash
kill `cat run.pid`
nohup ./go-loadbalancer > lb.log 2>&1 &
echo "$!" > run.pid
