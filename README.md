# Minimalist Netflow v5 to influxdb UDP collector written in Go 

Broker listens on specified UDP port (2055 by default), accepting Netflow traffic, and collecting records with selected metadata formatted in line protocol to UDP listener of [influxdb](https://github.com/influxdata/influxdb).

Project includes dockerfile for building runtime application as [docker](https://www.docker.com) container and also [Gitlab CI](https://about.gitlab.com/product/continuous-integration) definition file both for pushing build to docker registry and also for deploying to production docker server.

To enable UDP listener for influxdb, either modify `influxd.conf` file and add:

```
[[udp]]
enabled = true
bind-address = ":8089"
database = "db_for_netflow"
```

or set following environment variables:

```
INFLUXDB_UDP_ENABLED
INFLUXDB_UDP_BIND_ADDRESS
INFLUXDB_UDP_DATABASE
```

Netflow data in influxdb can then be analyzed for example with help of [Grafana](https://grafana.com).

**BEWARE!!!**

The cardinality of general Netflow data is quite high, so to use influxdb effectively for it, I suggest using several retention policies and downsampling the measurements.

PLAN FIRST! ;-)

## Usage

```
gonflux -method udp -out influxdb.local:8089
```

Supported command line parameters:
```
  -buffer int
        Size of RxQueue, i.e. value for SO_RCVBUF in bytes (default 212992)
  -in string
        Address and port to listen for NetFlow packets (default "0.0.0.0:2055")
  -method string
        Output method: stdout, udp (default "stdout")
  -out string
        Address and port of influxdb to send decoded data
```

Or with docker container: 

```
docker run --name=gonflux -t --restart=always -p 2055:2055/udp -e METHOD='udp' -e OUT='influxdb.local:8089' -d strzinek/gonflux
```

## Credits

This project was created with help of:

*  https://github.com/chemidy/smallest-secured-golang-docker-image
*  https://github.com/yunazuno/nfcollect

## Licence
**MIT**

Copyright (c) 2019 Pavel Strzinek

Permission is hereby granted, free of charge, to any person
obtaining a copy of this software and associated documentation
files (the "Software"), to deal in the Software without
restriction, including without limitation the rights to use,
copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the
Software is furnished to do so, subject to the following
conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
OTHER DEALINGS IN THE SOFTWARE.

