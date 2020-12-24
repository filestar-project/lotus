<p align="center">
  <a href="https://docs.filecoin.io/" title="Filecoin Docs">
    <img src="documentation/images/lotus_logo_h.png" alt="Project Lotus Logo" width="244" />
  </a>
</p>

<h1 align="center">Project Lotus - 莲</h1>

<p align="center">
  <a href="https://circleci.com/gh/filecoin-project/lotus"><img src="https://circleci.com/gh/filecoin-project/lotus.svg?style=svg"></a>
  <a href="https://codecov.io/gh/filecoin-project/lotus"><img src="https://codecov.io/gh/filecoin-project/lotus/branch/master/graph/badge.svg"></a>
  <a href="https://goreportcard.com/report/github.com/filecoin-project/lotus"><img src="https://goreportcard.com/badge/github.com/filecoin-project/lotus" /></a>  
  <a href=""><img src="https://img.shields.io/badge/golang-%3E%3D1.14.7-blue.svg" /></a>
  <br>
</p>

Lotus is an implementation of the Filecoin Distributed Storage Network. For more details about Filecoin, check out the [Filecoin Spec](https://spec.filecoin.io).

## Building & Documentation

For instructions on how to build, install and setup lotus, please visit [https://docs.filecoin.io/get-started/lotus](https://docs.filecoin.io/get-started/lotus/) and [https://github.com/filestar-project/docs/wiki](https://github.com/filestar-project/docs/wiki).

## Reporting a Vulnerability

Please send an email to dev@filestar.net. See our [security policy](SECURITY.md) for more details.

## Development

The main branches under development at the moment are:
* [`master`](https://github.com/filestar-project/lotus): current testnet.


### Packages

The lotus Filecoin implementation unfolds into the following packages:

- [This repo](https://github.com/filestar-project/lotus)
- [go-fil-markets](https://github.com/filecoin-project/go-fil-markets) which has its own [kanban work tracker available here](https://app.zenhub.com/workspaces/markets-shared-components-5daa144a7046a60001c6e253/board)
- [spec-actors](https://github.com/filestar-project/specs-actors) which has its own [kanban work tracker available here](https://app.zenhub.com/workspaces/actors-5ee6f3aa87591f0016c05685/board)

## License

Dual-licensed under [MIT](https://github.com/filestar-project/lotus/blob/master/LICENSE-MIT) + [Apache 2.0](https://github.com/filestar-project/lotus/blob/master/LICENSE-APACHE)
