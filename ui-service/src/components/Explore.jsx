'use client';

import React, { useEffect, useMemo, useRef, useState } from 'react';
import './GrafanaDashboard.css';

const DEFAULT_GRAFANA_PATH = '/grafana/explore';

const buildDefaultGrafanaPath = (deviceId = null) => {
    const params = new URLSearchParams({
        theme: 'light',
        orgId: '1',
        refresh: '10s',
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

    if (!grafanaUrl.searchParams.has('refresh')) {
        grafanaUrl.searchParams.set('refresh', '10s');
    }

    grafanaUrl.searchParams.delete('kiosk');

    if (deviceId) {
        grafanaUrl.searchParams.set('var-deviceId', deviceId);
    } else {
        grafanaUrl.searchParams.delete('var-deviceId');
    }

    return `${grafanaUrl.pathname}${grafanaUrl.search}${grafanaUrl.hash}`;
};

const GrafanaExplore = ({ deviceId = null }) => {
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

    const hideGrafanaNavigationOnly = () => {
        try {
            const iframe = iframeRef.current;
            const doc = iframe?.contentDocument || iframe?.contentWindow?.document;
            if (!doc) return;

            const styleId = 'taksa-hide-grafana-navigation-only';
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

                header:has(button[aria-label*="menu" i]),
                header:has(button[aria-label*="navigation" i]),
                header:has(a[href*="/grafana/login"]),
                header:has(a[href*="/login"]),
                header:has(nav[aria-label*="breadcrumb" i]),
                [role="banner"]:has(button[aria-label*="menu" i]),
                [role="banner"]:has(button[aria-label*="navigation" i]),
                [role="banner"]:has(a[href*="/grafana/login"]),
                [role="banner"]:has(a[href*="/login"]),
                [role="banner"]:has(nav[aria-label*="breadcrumb" i]) {
                    display: none !important;
                }

                a[href*="/grafana/login"],
                a[href*="/login"],
                button[aria-label*="help" i],
                button[aria-label*="search" i] {
                    display: none !important;
                }
            `;
        } catch (error) {
            console.warn('Could not hide Grafana navigation:', error);
        }
    };

    useEffect(() => {
        const iframe = iframeRef.current;
        if (!iframe) return;

        let observer = null;
        let menuInterval = null;

        const applyIframeTweaks = () => {
            hideGrafanaNavigationOnly();
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
                    title="Grafana Explore"
                    className="grafana-iframe"
                    frameBorder="0"
                    allowFullScreen
                    onLoad={hideGrafanaNavigationOnly}
                />
            </div>
        </div>
    );
};

export default GrafanaExplore;