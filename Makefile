# SPDX-License-Identifier: GPL-3.0-only
#
# This is free software, licensed under the GPL-3.0-only License.
#

include $(TOPDIR)/rules.mk

PKG_NAME:=cumtnet
PKG_VERSION:=1.0.0
PKG_RELEASE:=1

PKG_LICENSE:=GPL-3.0-only
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=Akinator365
PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1

# 设置 Go 包信息（可选，假设使用本地源码）
GO_PKG:=cumtnet
GO_PKG_LDFLAGS_X:=main.version=$(PKG_VERSION)

include $(INCLUDE_DIR)/package.mk

define Package/cumtnet
  TITLE:=CUMT Campus Network Auto Login
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=Web Servers/Proxies
  DEPENDS:=$(GO_ARCH_DEPENDS) +libc
  URL:=https://github.com/Akinator365/cumt-net
  USERID:=cumtnet:cumtnet
endef

define Package/cumtnet/description
  cumtnet is a lightweight tool to automatically log in to the CUMT campus network.
  It supports dynamic login and logout functionality for OpenWrt systems.
endef

# 构建逻辑
define Build/Compile
	$(GO_ENV_VARS) \
	CGO_ENABLED=0 go build -ldflags="-s -w -extldflags '-static'" -o $(PKG_BUILD_DIR)/cumtnet $(CURDIR)/cumtnet.go
endef

define Package/cumtnet/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/cumtnet $(1)/usr/bin/cumtnet

endef

define Package/$(PKG_NAME)/postinst
	#!/bin/sh

	echo '$(PKG_NAME) installed successed !'
	exit 0
endef

define Package/$(PKG_NAME)/prerm
	#!/bin/sh

	echo 'removeing $(PKG_NAME)'
	exit 0
endef

define Package/$(PKG_NAME)/postrm
	#!/bin/bash

	echo '$(PKG_NAME) remove successed !'
endef

$(eval $(call BuildPackage,cumtnet))
