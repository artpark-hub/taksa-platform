'use client';

import React, { useEffect, useMemo, useRef, useState } from 'react';
import './GrafanaDashboard.css';

const DEFAULT_GRAFANA_PATH = '/grafana/dashboards';

const GrafanaDashboard = ({ deviceId = null }) => {
    const iframeRef = useRef(null);
    const [iframeSrc, setIframeSrc] = useState('');

    const defaultSrc = useMemo(() => {
        const params = new URLSearchParams({
            theme: 'light',
            orgId: '1',
            refresh: '10s',
            ...(deviceId ? { 'var-deviceId': deviceId } : {}),
        });

        return `${DEFAULT_GRAFANA_PATH}?${params.toString()}`;
    }, [deviceId]);

    useEffect(() => {
        const url = new URL(window.location.href);
        const savedGrafanaPath = url.searchParams.get('grafana');

        if (savedGrafanaPath && savedGrafanaPath.startsWith('/grafana')) {
            setIframeSrc(savedGrafanaPath);
        } else {
            setIframeSrc(defaultSrc);
        }
    }, [defaultSrc]);

    useEffect(() => {
        const url = new URL(window.location.href);
        const savedGrafanaPath = url.searchParams.get('grafana');

        if (!savedGrafanaPath || !savedGrafanaPath.startsWith('/grafana')) {
            setIframeSrc(defaultSrc);
        }
    }, [deviceId, defaultSrc]);

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

                const parentUrl = new URL(window.location.href);
                const existingGrafanaPath = parentUrl.searchParams.get('grafana');

                if (existingGrafanaPath !== currentPath) {
                    parentUrl.searchParams.set('grafana', currentPath);
                    window.history.replaceState({}, '', parentUrl.toString());
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
    }, []);

    const hideGrafanaMenuButton = () => {
        try {
            const iframe = iframeRef.current;
            const doc = iframe?.contentDocument || iframe?.contentWindow?.document;
            if (!doc) return;

            const styleId = 'taksa-hide-grafana-menu-button';
            let style = doc.getElementById(styleId);

            if (!style) {
                style = doc.createElement('style');
                style.id = styleId;
                doc.head.appendChild(style);
            }

            style.textContent = `
                button[aria-label*="menu" i],
                button[aria-label*="navigation" i],
                [data-testid="sidemenu-toggle"],
                [data-testid="nav-toggle"],
                [data-testid="menu-toggle"] {
                    display: none !important;
                }
            `;
        } catch (error) {
            console.warn('Could not hide Grafana menu button:', error);
        }
    };

    useEffect(() => {
        const iframe = iframeRef.current;
        if (!iframe) return;

        let observer = null;
        let menuInterval = null;

        const applyIframeTweaks = () => {
            hideGrafanaMenuButton();
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
                    onLoad={hideGrafanaMenuButton}
                />
            </div>
        </div>
    );
};

export default GrafanaDashboard;