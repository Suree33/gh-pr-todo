# Changelog

## [0.5.2](https://github.com/Suree33/gh-pr-todo/compare/v0.5.1...v0.5.2) (2026-04-28)


### Miscellaneous Chores

* **deps:** bump github.com/odvcencio/gotreesitter from 0.12.2 to 0.15.2 ([#58](https://github.com/Suree33/gh-pr-todo/issues/58)) ([7b82bd7](https://github.com/Suree33/gh-pr-todo/commit/7b82bd71335fd8f5965a74e0d2dfd01c52050cd7))
* update release-please config to refine sections ([#56](https://github.com/Suree33/gh-pr-todo/issues/56)) ([4a86e68](https://github.com/Suree33/gh-pr-todo/commit/4a86e6854238c37b6298d5b7260fd898b1acf59e))

## [0.5.1](https://github.com/Suree33/gh-pr-todo/compare/v0.5.0...v0.5.1) (2026-04-23)


### Documentation

* **README:** add downloads badge ([#43](https://github.com/Suree33/gh-pr-todo/issues/43)) ([62f070b](https://github.com/Suree33/gh-pr-todo/commit/62f070bdd3e55966e91370198a96a58f2ec6e050))


### Refactoring

* split main.go into internal/github, internal/output, and parser helpers ([#52](https://github.com/Suree33/gh-pr-todo/issues/52)) ([8dae986](https://github.com/Suree33/gh-pr-todo/commit/8dae9866ad4550f5ceb4e0e861cabc96cdde733f))


### Miscellaneous Chores

* **deps:** bump googleapis/release-please-action from 4 to 4.4.1 ([#51](https://github.com/Suree33/gh-pr-todo/issues/51)) ([de000e5](https://github.com/Suree33/gh-pr-todo/commit/de000e573be15647789ca0f9534b55c76ab2d7cf))
* **release:** revert PR [#37](https://github.com/Suree33/gh-pr-todo/issues/37) ([#45](https://github.com/Suree33/gh-pr-todo/issues/45)) ([4d6b6b4](https://github.com/Suree33/gh-pr-todo/commit/4d6b6b4595a2b7ab222fa00cef4bc76a004f5df9))

## [0.5.0](https://github.com/Suree33/gh-pr-todo/compare/v0.4.1...v0.5.0) (2026-04-01)


### Features

* **parser:** add Tree-sitter integration for syntax-aware TODO detection ([#40](https://github.com/Suree33/gh-pr-todo/issues/40)) ([0097b7c](https://github.com/Suree33/gh-pr-todo/commit/0097b7cf996f032af9fb7db90233111a63e064d6))


### Documentation

* add AGENTS.md ([#38](https://github.com/Suree33/gh-pr-todo/issues/38)) ([049f880](https://github.com/Suree33/gh-pr-todo/commit/049f880824ce9eebabb4dfd36d90beabbaa60f0a))


### Miscellaneous Chores

* remove bump-patch-for-minor-pre-major from release-please config ([#42](https://github.com/Suree33/gh-pr-todo/issues/42)) ([c7d0f14](https://github.com/Suree33/gh-pr-todo/commit/c7d0f14f19d17abf5d09d56a9aa6006fd1bcbf9f))

## [0.4.1](https://github.com/Suree33/gh-pr-todo/compare/v0.4.0...v0.4.1) (2026-03-30)


### CI

* refactor ([#29](https://github.com/Suree33/gh-pr-todo/issues/29)) ([d7fe849](https://github.com/Suree33/gh-pr-todo/commit/d7fe849a7110a54ae87de8389f79ae1106f355cc))


### Miscellaneous Chores

* add release-please ([#34](https://github.com/Suree33/gh-pr-todo/issues/34)) ([41e9b81](https://github.com/Suree33/gh-pr-todo/commit/41e9b813ea64b701d5a7067810142e66b1d06898))
* **ci:** add changelog-sections to release-please-config ([#36](https://github.com/Suree33/gh-pr-todo/issues/36)) ([9615d2d](https://github.com/Suree33/gh-pr-todo/commit/9615d2dbb6bbcce4eb98182a248063a269eb0cce))
* **dependabot:** add github-actions ([#31](https://github.com/Suree33/gh-pr-todo/issues/31)) ([77dc50f](https://github.com/Suree33/gh-pr-todo/commit/77dc50f8084d068753bb1159c001102900cf32ee))
* **dependabot:** remove cooldown.semver-major-days ([53e1087](https://github.com/Suree33/gh-pr-todo/commit/53e1087d3e9819668a486254044144e9e3c507aa))
* **deps:** bump actions/checkout from 5 to 6 ([#33](https://github.com/Suree33/gh-pr-todo/issues/33)) ([63cf10f](https://github.com/Suree33/gh-pr-todo/commit/63cf10fd9f70e3c8d3fe022a9101091eaa63ed11))
* **deps:** bump github.com/cli/go-gh/v2 from 2.12.2 to 2.13.0 ([8ad4099](https://github.com/Suree33/gh-pr-todo/commit/8ad40996e81b0f56db96496299950995511e39b2))
* **deps:** bump github.com/cli/go-gh/v2 from 2.12.2 to 2.13.0 ([eff1adc](https://github.com/Suree33/gh-pr-todo/commit/eff1adcff8d135e1bf69c900b62f369ba77424cb))
* **deps:** bump github.com/fatih/color from 1.18.0 to 1.19.0 ([#30](https://github.com/Suree33/gh-pr-todo/issues/30)) ([cbb4e1c](https://github.com/Suree33/gh-pr-todo/commit/cbb4e1cd7dd21fba2f2c9ac3e4d36066c1566a1e))
* replace github_token with GitHub App Token ([#37](https://github.com/Suree33/gh-pr-todo/issues/37)) ([a199229](https://github.com/Suree33/gh-pr-todo/commit/a199229746d95ffac7edf921c8dbedbee66706a5))

## [0.4.0](https://github.com/Suree33/gh-pr-todo/releases/tag/v0.4.0) (2025-11-08)

### Features

- improve error handling ([#25](https://github.com/Suree33/gh-pr-todo/pull/25))

## [0.3.1](https://github.com/Suree33/gh-pr-todo/releases/tag/v0.3.1) (2025-10-09)

### Miscellaneous Chores

- bump `github.com/spf13/pflag` from `1.0.7` to `1.0.10` ([#22](https://github.com/Suree33/gh-pr-todo/pull/22))
- update dependabot configuration to run daily and set cooldown periods ([#23](https://github.com/Suree33/gh-pr-todo/pull/23))

## [0.3.0](https://github.com/Suree33/gh-pr-todo/releases/tag/v0.3.0) (2025-09-05)

### Features

- add `--group-by` ([#20](https://github.com/Suree33/gh-pr-todo/pull/20))

### Miscellaneous Chores

- upgrade Go ([#21](https://github.com/Suree33/gh-pr-todo/pull/21))

## [0.2.1](https://github.com/Suree33/gh-pr-todo/releases/tag/v0.2.1) (2025-08-26)

### Bug Fixes

- show `0` when no TODOs are found ([#18](https://github.com/Suree33/gh-pr-todo/pull/18))

## [0.2.0](https://github.com/Suree33/gh-pr-todo/releases/tag/v0.2.0) (2025-08-26)

### Features

- add `--name-only` ([#15](https://github.com/Suree33/gh-pr-todo/pull/15))
- add `--count` and fix usage ([#16](https://github.com/Suree33/gh-pr-todo/pull/16))

### Documentation

- add badges to `README` ([#12](https://github.com/Suree33/gh-pr-todo/pull/12))
- align `README` with implementation ([#14](https://github.com/Suree33/gh-pr-todo/pull/14))
- update `README` ([#17](https://github.com/Suree33/gh-pr-todo/pull/17))

### CI

- add permissions to CI workflow ([#13](https://github.com/Suree33/gh-pr-todo/pull/13))

## [0.1.0](https://github.com/Suree33/gh-pr-todo/releases/tag/v0.1.0) (2025-08-21)

### Features

- add flags to specify repository and pull request ([#8](https://github.com/Suree33/gh-pr-todo/pull/8))

### Documentation

- add command line argument documentation to `README` ([#9](https://github.com/Suree33/gh-pr-todo/pull/9))

### Refactoring

- replace `flag` with `pflag` and improve argument validation ([#10](https://github.com/Suree33/gh-pr-todo/pull/10))

### CI

- add CI workflow ([#7](https://github.com/Suree33/gh-pr-todo/pull/7))

## [0.0.2](https://github.com/Suree33/gh-pr-todo/releases/tag/v0.0.2) (2025-08-19)

### Miscellaneous Chores

- migrate Go version ([#2](https://github.com/Suree33/gh-pr-todo/pull/2))
- add Dependabot version updates ([#3](https://github.com/Suree33/gh-pr-todo/pull/3))

## [0.0.1](https://github.com/Suree33/gh-pr-todo/releases/tag/v0.0.1) (2025-08-19)

### Features

- implement main feature ([#1](https://github.com/Suree33/gh-pr-todo/pull/1))
