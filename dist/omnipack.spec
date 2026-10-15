Name:           omnipack
Version:        1.0.0
Release:        1%{?dist}
Summary:        Universal Linux Application & Package Manager

License:        MIT
URL:            https://github.com/Sushant0999/linux_package_manager
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  go >= 1.22
BuildRequires:  npm
BuildRequires:  gcc
BuildRequires:  pkgconfig(webkit2gtk-4.1)
BuildRequires:  pkgconfig(gtk+-3.0)

Requires:       webkit2gtk4.1
Requires:       gtk3
Recommends:     flatpak
Recommends:     xdg-utils

%description
Omnipack is a modern GUI application for managing software on any Linux
distribution. Install apps from URLs, .tar.gz, .zip, .AppImage files,
and system repositories (dnf, apt, pacman, flatpak) — all from one
unified interface.

%prep
%setup -q

%build
export PATH="$PATH:$(go env GOPATH)/bin"
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 || true
wails build -tags webkit2_41 -o omnipack

%install
rm -rf %{buildroot}

install -Dm755 build/bin/omnipack       %{buildroot}%{_bindir}/omnipack
install -Dm644 dist/omnipack.desktop    %{buildroot}%{_datadir}/applications/omnipack.desktop
install -Dm644 dist/icons/omnipack.png  %{buildroot}%{_datadir}/pixmaps/omnipack.png
install -Dm644 dist/icons/omnipack.png  %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/omnipack.png
install -Dm644 dist/icons/omnipack.png  %{buildroot}%{_datadir}/icons/hicolor/512x512/apps/omnipack.png

%post
gtk-update-icon-cache -f -t %{_datadir}/icons/hicolor &>/dev/null || :
update-desktop-database %{_datadir}/applications &>/dev/null || :

%postun
gtk-update-icon-cache -f -t %{_datadir}/icons/hicolor &>/dev/null || :
update-desktop-database %{_datadir}/applications &>/dev/null || :

%files
%{_bindir}/omnipack
%{_datadir}/applications/omnipack.desktop
%{_datadir}/pixmaps/omnipack.png
%{_datadir}/icons/hicolor/256x256/apps/omnipack.png
%{_datadir}/icons/hicolor/512x512/apps/omnipack.png

%changelog
* Sun Oct 04 2026 Sushant Raj <sushant0999@github.com> - 1.0.0-1
- Initial release
