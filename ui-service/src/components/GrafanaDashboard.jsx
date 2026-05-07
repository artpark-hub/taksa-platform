'use client';

import React, { useEffect, useMemo, useRef, useState } from 'react';
import './GrafanaDashboard.css';

const DEFAULT_GRAFANA_PATH = '/grafana/dashboards';

const isGrafanaEditorPath = (grafanaUrl) => {
    return (
        grafanaUrl.pathname === '/grafana/dashboard/new' ||
        grafanaUrl.searchParams.has('editPanel')
    );
};

const buildDefaultGrafanaPath = (deviceId = null) => {
    const params = new URLSearchParams({
        theme: 'light',
        orgId: '1',
        refresh: '10s',
        kiosk: '1',
        ...(deviceId ? { 'var-deviceId': deviceId } : {}),
    });

    return `${DEFAULT_GRAFANA_PATH}?${params.toString()}`;
};

const getGrafanaPathWithParams = (path, deviceId = null) => {
    const grafanaUrl = new URL(path, window.location.origin);

    if (!grafanaUrl.pathname.startsWith('/grafana')) {
        return path;
    }

    if (!grafanaUrl.searchParams.has('theme')) {
        grafanaUrl.searchParams.set('theme', 'light');
    }

    if (!grafanaUrl.searchParams.has('orgId')) {
        grafanaUrl.searchParams.set('orgId', '1');
    }

    if (isGrafanaEditorPath(grafanaUrl)) {
        grafanaUrl.searchParams.delete('kiosk');
        grafanaUrl.searchParams.delete('refresh');
    } else {
        if (!grafanaUrl.searchParams.has('refresh')) {
            grafanaUrl.searchParams.set('refresh', '10s');
        }

        grafanaUrl.searchParams.set('kiosk', '1');
    }

    if (deviceId) {
        grafanaUrl.searchParams.set('var-deviceId', deviceId);
    } else {
        grafanaUrl.searchParams.delete('var-deviceId');
    }

    return `${grafanaUrl.pathname}${grafanaUrl.search}${grafanaUrl.hash}`;
};

const GrafanaDashboard = ({ deviceId = null }) => {
    const iframeRef = useRef(null);
    const [iframeSrc, setIframeSrc] = useState('');

    const defaultSrc = useMemo(() => {
        return buildDefaultGrafanaPath(deviceId);
    }, [deviceId]);

    useEffect(() => {
        const url = new URL(window.location.href);
        const savedGrafanaPath = url.searchParams.get('grafana');

        if (savedGrafanaPath && savedGrafanaPath.startsWith('/grafana')) {
            setIframeSrc(getGrafanaPathWithParams(savedGrafanaPath, deviceId));
        } else {
            setIframeSrc(defaultSrc);
        }
    }, [defaultSrc, deviceId]);

    useEffect(() => {
        const url = new URL(window.location.href);
        const savedGrafanaPath = url.searchParams.get('grafana');

        if (!savedGrafanaPath || !savedGrafanaPath.startsWith('/grafana')) {
            setIframeSrc(defaultSrc);
        }
    }, [deviceId, defaultSrc]);

    useEffect(() => {
        const syncIframeFromParentUrl = () => {
            const url = new URL(window.location.href);
            const savedGrafanaPath = url.searchParams.get('grafana');

            if (savedGrafanaPath && savedGrafanaPath.startsWith('/grafana')) {
                setIframeSrc(getGrafanaPathWithParams(savedGrafanaPath, deviceId));
                return;
            }

            setIframeSrc(defaultSrc);
        };

        window.addEventListener('popstate', syncIframeFromParentUrl);

        return () => {
            window.removeEventListener('popstate', syncIframeFromParentUrl);
        };
    }, [defaultSrc, deviceId]);

    useEffect(() => {
        let interval = null;
        let crossOriginBlocked = false;

        const syncParentUrlWithIframe = () => {
            if (crossOriginBlocked) return;

            try {
                const iframeWindow = iframeRef.current?.contentWindow;
                if (!iframeWindow) return;

                const currentPath =
                    iframeWindow.location.pathname +
                    iframeWindow.location.search +
                    iframeWindow.location.hash;

                if (!currentPath.startsWith('/grafana')) return;

                const sanitizedPath = getGrafanaPathWithParams(currentPath, deviceId);

                if (sanitizedPath !== currentPath) {
                    setIframeSrc(sanitizedPath);
                    return;
                }

                const parentUrl = new URL(window.location.href);
                const existingGrafanaPath = parentUrl.searchParams.get('grafana');

                if (existingGrafanaPath !== sanitizedPath) {
                    parentUrl.searchParams.set('grafana', sanitizedPath);
                    window.history.pushState({}, '', parentUrl.toString());
                }
            } catch {
                crossOriginBlocked = true;
                if (interval) clearInterval(interval);
                interval = null;
            }
        };

        interval = setInterval(syncParentUrlWithIframe, 300);

        return () => {
            if (interval) clearInterval(interval);
        };
    }, [deviceId]);

    const hideGrafanaChrome = () => {
        try {
            const iframe = iframeRef.current;
            const doc = iframe?.contentDocument || iframe?.contentWindow?.document;
            if (!doc) return;

            const styleId = 'taksa-hide-grafana-chrome';
            let style = doc.getElementById(styleId);

            if (!style) {
                style = doc.createElement('style');
                style.id = styleId;
                doc.head.appendChild(style);
            }

            style.textContent = `
                [data-testid="sidemenu"],
                [data-testid="sidemenu-toggle"],
                [data-testid="nav-toggle"],
                [data-testid="menu-toggle"],
                nav[aria-label*="navigation" i],
                nav[aria-label*="main" i],
                [class*="sidemenu"],
                [class*="SideMenu"],
                [class*="powered-by"],
                a[href*="grafana.com"],
                [role="contentinfo"] {
                    display: none !important;
                }

                header:has(a[href*="/grafana/dashboards"]),
                header:has(a[href*="/dashboards"]),
                header:has(a[href*="/grafana/login"]),
                header:has(a[href*="/login"]),
                header:has(nav[aria-label*="breadcrumb" i]),
                header:has(button[aria-label*="search" i]),
                [role="banner"]:has(a[href*="/grafana/dashboards"]),
                [role="banner"]:has(a[href*="/dashboards"]),
                [role="banner"]:has(a[href*="/grafana/login"]),
                [role="banner"]:has(a[href*="/login"]),
                [role="banner"]:has(nav[aria-label*="breadcrumb" i]),
                [role="banner"]:has(button[aria-label*="search" i]),
                [data-testid="page-header"],
                [data-testid="page-toolbar"],
                [data-testid="topnav"],
                [data-testid="top-nav"],
                [data-testid="nav-toolbar"],
                [class*="TopNav"],
                [class*="topnav"],
                [class*="PageToolbar"],
                [class*="page-toolbar"] {
                    display: none !important;
                }

                header nav[aria-label*="breadcrumb" i],
                header a[href*="/grafana/dashboards"],
                header a[href*="/dashboards"],
                header a[href*="/grafana/login"],
                header a[href*="/login"],
                header button[aria-label*="search" i],
                header button[aria-label*="create" i],
                header button[aria-label*="new" i],
                header button[aria-label*="help" i],
                header [aria-label*="sign in" i],
                header [data-testid*="breadcrumb" i],
                [role="banner"] nav[aria-label*="breadcrumb" i],
                [role="banner"] a[href*="/grafana/dashboards"],
                [role="banner"] a[href*="/dashboards"],
                [role="banner"] a[href*="/grafana/login"],
                [role="banner"] a[href*="/login"],
                [role="banner"] button[aria-label*="search" i],
                [role="banner"] button[aria-label*="create" i],
                [role="banner"] button[aria-label*="new" i],
                [role="banner"] button[aria-label*="help" i],
                [role="banner"] [aria-label*="sign in" i],
                [role="banner"] [data-testid*="breadcrumb" i] {
                    display: none !important;
                }
            `;
        } catch (error) {
            console.warn('Could not hide Grafana chrome:', error);
        }
    };

    useEffect(() => {
        const iframe = iframeRef.current;
        if (!iframe) return;

        let observer = null;
        let menuInterval = null;

        const applyIframeTweaks = () => {
            hideGrafanaChrome();
        };

        const handleLoad = () => {
            applyIframeTweaks();

            try {
                const doc = iframe.contentDocument || iframe.contentWindow?.document;
                if (!doc?.body) return;

                if (observer) observer.disconnect();

                observer = new MutationObserver(() => {
                    applyIframeTweaks();
                });

                observer.observe(doc.body, {
                    childList: true,
                    subtree: true,
                    attributes: true,
                });

                if (menuInterval) clearInterval(menuInterval);
                menuInterval = setInterval(applyIframeTweaks, 500);
            } catch (error) {
                console.warn('Could not observe Grafana DOM:', error);
            }
        };

        iframe.addEventListener('load', handleLoad);

        return () => {
            iframe.removeEventListener('load', handleLoad);
            if (observer) observer.disconnect();
            if (menuInterval) clearInterval(menuInterval);
        };
    }, [iframeSrc]);

    if (!iframeSrc) return null;

    return (
        <div className="grafana-wrapper">
            <div className="grafana-frame-container">
                <iframe
                    ref={iframeRef}
                    key={deviceId || 'default'}
                    src={iframeSrc}
                    title="Grafana Dashboard"
                    className="grafana-iframe"
                    frameBorder="0"
                    allowFullScreen
                    onLoad={hideGrafanaChrome}
                />
            </div>
        </div>
    );
};

export default GrafanaDashboard;