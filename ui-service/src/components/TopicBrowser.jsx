'use client';

import React, { useEffect, useMemo, useState } from 'react';
import {
    ArrowLeft,
    ChevronDown,
    ChevronRight,
    Clock,
    Database,
    FileText,
    FolderTree,
    Search,
    Server
} from 'lucide-react';
import { useRouter, useSearchParams } from 'next/navigation';
import SelectDCD from './SelectDCD';
import './TopicBrowser.css';

const getErrorMessage = (data, fallback) => (
    data?.error?.message ||
    data?.message ||
    data?.details ||
    fallback
);

const getPathKey = (path = []) => path.join('/');

const toTopicPathSegments = (topic) => String(topic || '')
    .split('.')
    .map((segment) => segment.trim())
    .filter(Boolean);

const formatTopicPath = (path = []) => path.join('/');

const getMetadataValue = (detail, key) => {
    if (!Array.isArray(detail?.metadata)) {
        return '';
    }

    const entry = detail.metadata.find((item) => item?.key === key);
    return entry?.value ?? '';
};

const unwrapDetailResponse = (data) => data?.result || data?.payload || data || {};

const formatIstDateTime = (value) => {
    if (!value) {
        return '-';
    }

    const date = new Date(value);

    if (Number.isNaN(date.getTime())) {
        return String(value);
    }

    return `${new Intl.DateTimeFormat('en-IN', {
        timeZone: 'Asia/Kolkata',
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    }).format(date)} IST`;
};

const getTopicLocationPath = (detail, fallbackTopic) => {
    const locationParts = [
        detail?.level0,
        ...(Array.isArray(detail?.locationSublevels) ? detail.locationSublevels : [])
    ].filter(Boolean);

    if (locationParts.length) {
        return locationParts.join('/');
    }

    const metadataLocationPath = getMetadataValue(detail, 'location_path');

    if (metadataLocationPath) {
        return String(metadataLocationPath).replaceAll('.', '/');
    }

    const topicSegments = toTopicPathSegments(detail?.topic || fallbackTopic?.canonicalTopic || '');
    const dataContract = detail?.dataContract || getMetadataValue(detail, 'data_contract');
    const dataContractIndex = dataContract ? topicSegments.findIndex((segment) => segment === dataContract) : -1;

    if (dataContractIndex > 0) {
        return topicSegments.slice(0, dataContractIndex).join('/');
    }

    if (topicSegments.length > 2) {
        return topicSegments.slice(0, -2).join('/');
    }

    return '-';
};

const getTagName = (detail, fallbackTopic) => (
    detail?.name ||
    getMetadataValue(detail, 'tag_name') ||
    fallbackTopic?.segment ||
    toTopicPathSegments(detail?.topic || fallbackTopic?.canonicalTopic || '').at(-1) ||
    '-'
);

const getTopicDataContract = (detail, fallbackTopic) => {
    const directDataContract = detail?.dataContract || getMetadataValue(detail, 'data_contract');

    if (directDataContract) {
        return directDataContract;
    }

    const topicSegments = toTopicPathSegments(detail?.topic || fallbackTopic?.canonicalTopic || '');
    return topicSegments.find((segment) => segment.startsWith('_')) || '-';
};

const getLastMessageSummary = (detail) => {
    if (detail?.timeSeries) {
        return {
            payloadType: 'Time-Series',
            producedAt: formatIstDateTime(detail.timeSeries.producedAt),
            type: detail.timeSeries.scalarType ? String(detail.timeSeries.scalarType).toLowerCase() : '-',
            updatedAt: formatIstDateTime(detail.updatedAt)
        };
    }

    if (detail?.relational) {
        return {
            payloadType: 'Relational',
            producedAt: formatIstDateTime(detail.relational.producedAt),
            type: detail.relational.type || '-',
            updatedAt: formatIstDateTime(detail.updatedAt)
        };
    }

    return {
        payloadType: '-',
        producedAt: '-',
        type: '-',
        updatedAt: '-'
    };
};

const mapApiNodes = (nodes = [], prefix = []) => nodes.map((node) => {
    const segment = String(node?.segment || node?.name || '').trim();
    const path = [...prefix, segment];
    const isLeaf = Boolean(node?.isLeaf);

    return {
        segment,
        path,
        isLeaf,
        descendantLeafCount: Number(node?.descendantLeafCount || 0),
        unsTreeId: node?.unsTreeId || '',
        canonicalTopic: node?.canonicalTopic || node?.topic || '',
        children: [],
        childrenLoaded: isLeaf,
        isLoading: false,
        error: ''
    };
}).filter((node) => node.segment);

const updateNodeByPath = (nodes, targetPath, updater) => {
    const targetKey = getPathKey(targetPath);

    return nodes.map((node) => {
        if (getPathKey(node.path) === targetKey) {
            return updater(node);
        }

        if (node.children?.length) {
            return {
                ...node,
                children: updateNodeByPath(node.children, targetPath, updater)
            };
        }

        return node;
    });
};

const DetailRow = ({ label, value }) => (
    <div className="topic-browser-detail-row">
        <span>{label}</span>
        <strong>{value || '-'}</strong>
    </div>
);

const MessageRow = ({ icon: Icon, label, value }) => (
    <div className="topic-browser-message-row">
        <Icon size={18} />
        <span>{label}</span>
        <strong>{value || '-'}</strong>
    </div>
);

const TopicBrowser = () => {
    const router = useRouter();
    const searchParams = useSearchParams();
    const deviceId = searchParams?.get('deviceId') || '';
    const deviceName = searchParams?.get('deviceName') || '';

    const [catalogStatus, setCatalogStatus] = useState(null);
    const [isCatalogLoading, setIsCatalogLoading] = useState(false);
    const [catalogError, setCatalogError] = useState('');
    const [rootNodes, setRootNodes] = useState([]);
    const [isTreeLoading, setIsTreeLoading] = useState(false);
    const [treeError, setTreeError] = useState('');
    const [expandedPaths, setExpandedPaths] = useState({});
    const [selectedTopic, setSelectedTopic] = useState(null);
    const [topicDetail, setTopicDetail] = useState(null);
    const [topicDetailRaw, setTopicDetailRaw] = useState(null);
    const [isDetailLoading, setIsDetailLoading] = useState(false);
    const [detailError, setDetailError] = useState('');
    const [isMetadataOpen, setIsMetadataOpen] = useState(false);
    const [searchTerm, setSearchTerm] = useState('');
    const [searchResults, setSearchResults] = useState([]);
    const [isSearchLoading, setIsSearchLoading] = useState(false);
    const [searchError, setSearchError] = useState('');

    const catalogReady = catalogStatus?.lastSyncMode === 'FULL_REPLACE';
    const selectedTopicPath = useMemo(() => {
        if (!selectedTopic && !topicDetail) {
            return '';
        }

        const canonicalTopic = topicDetail?.topic || selectedTopic?.canonicalTopic;

        if (canonicalTopic) {
            return formatTopicPath(toTopicPathSegments(canonicalTopic));
        }

        return formatTopicPath(selectedTopic?.path || []);
    }, [selectedTopic, topicDetail]);

    const fetchNodes = async (pathPrefix = []) => {
        const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(deviceId)}/topics/nodes/list`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Accept: 'application/json'
            },
            credentials: 'include',
            body: JSON.stringify({
                deviceId,
                pathPrefix
            })
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok) {
            throw new Error(getErrorMessage(data, 'Failed to load topic nodes.'));
        }

        return mapApiNodes(Array.isArray(data?.nodes) ? data.nodes : [], pathPrefix);
    };

    const loadRootNodes = async () => {
        try {
            setIsTreeLoading(true);
            setTreeError('');
            setSelectedTopic(null);
            setTopicDetail(null);
            setTopicDetailRaw(null);
            setDetailError('');
            setIsMetadataOpen(false);
            setExpandedPaths({});
            const nodes = await fetchNodes([]);
            setRootNodes(nodes);
        } catch (error) {
            console.error('Failed to load root topic nodes:', error);
            setTreeError(error.message || 'Failed to load topic nodes.');
            setRootNodes([]);
        } finally {
            setIsTreeLoading(false);
        }
    };

    const loadTopicDetail = async (topic) => {
        setSelectedTopic(topic);
        setTopicDetail(null);
        setTopicDetailRaw(null);
        setDetailError('');
        setIsMetadataOpen(false);

        const params = new URLSearchParams();

        if (topic?.unsTreeId) {
            params.set('unsTreeId', topic.unsTreeId);
        } else if (topic?.canonicalTopic) {
            params.set('canonicalTopic', topic.canonicalTopic);
        } else if (topic?.path?.length) {
            params.set('canonicalTopic', topic.path.join('.'));
        }

        if (!params.toString()) {
            return;
        }

        try {
            setIsDetailLoading(true);

            const response = await fetch(
                `/api/v1/devicemgmt/devices/${encodeURIComponent(deviceId)}/topics/detail?${params.toString()}`,
                {
                    method: 'GET',
                    headers: {
                        Accept: 'application/json'
                    },
                    credentials: 'include'
                }
            );
            const data = await response.json().catch(() => ({}));

            if (!response.ok) {
                throw new Error(getErrorMessage(data, 'Failed to load topic details.'));
            }

            setTopicDetailRaw(data);
            setTopicDetail(unwrapDetailResponse(data));
        } catch (error) {
            console.error('Failed to load topic detail:', error);
            setDetailError(error.message || 'Failed to load topic details.');
        } finally {
            setIsDetailLoading(false);
        }
    };

    useEffect(() => {
        if (!deviceId) {
            return undefined;
        }

        let cancelled = false;
        let retryTimer = null;

        const loadCatalogStatus = async () => {
            try {
                setIsCatalogLoading(true);
                setCatalogError('');

                const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(deviceId)}/topics/catalog-status`, {
                    method: 'GET',
                    headers: {
                        Accept: 'application/json'
                    },
                    credentials: 'include'
                });

                const data = await response.json().catch(() => ({}));

                if (!response.ok) {
                    throw new Error(getErrorMessage(data, 'Failed to load topic catalog status.'));
                }

                if (cancelled) {
                    return;
                }

                setCatalogStatus(data);

                if (data?.lastSyncMode === 'FULL_REPLACE') {
                    await loadRootNodes();
                } else {
                    retryTimer = setTimeout(loadCatalogStatus, 3000);
                }
            } catch (error) {
                if (!cancelled) {
                    console.error('Failed to load topic catalog status:', error);
                    setCatalogError(error.message || 'Failed to load topic catalog status.');
                }
            } finally {
                if (!cancelled) {
                    setIsCatalogLoading(false);
                }
            }
        };

        setCatalogStatus(null);
        setRootNodes([]);
        setSelectedTopic(null);
        setTopicDetail(null);
        setTopicDetailRaw(null);
        setDetailError('');
        setIsMetadataOpen(false);
        setSearchTerm('');
        setSearchResults([]);
        loadCatalogStatus();

        return () => {
            cancelled = true;
            if (retryTimer) {
                clearTimeout(retryTimer);
            }
        };
    }, [deviceId]);

    useEffect(() => {
        if (!deviceId || !catalogReady) {
            return undefined;
        }

        const typedText = searchTerm.trim();

        if (!typedText) {
            setSearchResults([]);
            setSearchError('');
            setIsSearchLoading(false);
            return undefined;
        }

        let cancelled = false;
        const searchTimer = setTimeout(async () => {
            try {
                setIsSearchLoading(true);
                setSearchError('');

                const response = await fetch(`/api/v1/devicemgmt/devices/${encodeURIComponent(deviceId)}/topics/list`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        Accept: 'application/json'
                    },
                    credentials: 'include',
                    body: JSON.stringify({
                        deviceId,
                        text: typedText,
                        pageSize: 20,
                        pageToken: ''
                    })
                });

                const data = await response.json().catch(() => ({}));

                if (!response.ok) {
                    throw new Error(getErrorMessage(data, 'Failed to search topics.'));
                }

                if (!cancelled) {
                    setSearchResults(Array.isArray(data?.topics) ? data.topics : []);
                }
            } catch (error) {
                if (!cancelled) {
                    console.error('Failed to search topics:', error);
                    setSearchError(error.message || 'Failed to search topics.');
                    setSearchResults([]);
                }
            } finally {
                if (!cancelled) {
                    setIsSearchLoading(false);
                }
            }
        }, 300);

        return () => {
            cancelled = true;
            clearTimeout(searchTimer);
        };
    }, [catalogReady, deviceId, searchTerm]);

    const handleToggleNode = async (node) => {
        if (node.isLeaf) {
            await loadTopicDetail(node);
            return;
        }

        const pathKey = getPathKey(node.path);
        const isExpanded = Boolean(expandedPaths[pathKey]);

        setExpandedPaths((prev) => ({
            ...prev,
            [pathKey]: !isExpanded
        }));

        if (isExpanded || node.childrenLoaded || node.isLoading) {
            return;
        }

        setRootNodes((prev) => updateNodeByPath(prev, node.path, (currentNode) => ({
            ...currentNode,
            isLoading: true,
            error: ''
        })));

        try {
            const children = await fetchNodes(node.path);
            setRootNodes((prev) => updateNodeByPath(prev, node.path, (currentNode) => ({
                ...currentNode,
                children,
                childrenLoaded: true,
                isLoading: false,
                error: ''
            })));
        } catch (error) {
            console.error('Failed to load child topic nodes:', error);
            setRootNodes((prev) => updateNodeByPath(prev, node.path, (currentNode) => ({
                ...currentNode,
                isLoading: false,
                error: error.message || 'Failed to load child topics.'
            })));
        }
    };

    const handleSearchResultSelect = (topic) => {
        const canonicalTopic = topic?.topic || topic?.canonicalTopic || '';
        const path = toTopicPathSegments(canonicalTopic);

        loadTopicDetail({
            segment: topic?.name || path[path.length - 1] || canonicalTopic || 'Topic',
            path,
            isLeaf: true,
            unsTreeId: topic?.unsTreeId || '',
            canonicalTopic,
            topicRow: topic
        });
    };

    const renderTreeNodes = (nodes, depth = 0) => nodes.map((node) => {
        const pathKey = getPathKey(node.path);
        const expanded = Boolean(expandedPaths[pathKey]);
        const selected = selectedTopicPath === formatTopicPath(node.path) || selectedTopic?.unsTreeId === node.unsTreeId;

        return (
            <div key={pathKey} className="topic-browser-tree-group">
                <button
                    type="button"
                    className={`topic-browser-tree-node${selected ? ' active' : ''}${node.isLeaf ? ' leaf' : ''}`}
                    style={{ '--tree-depth': depth }}
                    onClick={() => handleToggleNode(node)}
                >
                    {node.isLeaf ? (
                        <span className="topic-browser-leaf-dot" />
                    ) : expanded ? (
                        <ChevronDown size={18} />
                    ) : (
                        <ChevronRight size={18} />
                    )}
                    <span>{node.segment}</span>
                    {!node.isLeaf && node.descendantLeafCount > 0 && (
                        <span className="topic-browser-count-pill">{node.descendantLeafCount}</span>
                    )}
                </button>

                {node.error && <p className="topic-browser-node-error">{node.error}</p>}
                {node.isLoading && <p className="topic-browser-node-loading">Loading...</p>}

                {!node.isLeaf && expanded && node.children?.length > 0 && (
                    <div className="topic-browser-tree-children">
                        {renderTreeNodes(node.children, depth + 1)}
                    </div>
                )}
            </div>
        );
    });

    const renderSearchResults = () => {
        if (isSearchLoading) {
            return <p className="topic-browser-tree-status">Searching topics...</p>;
        }

        if (searchError) {
            return <p className="topic-browser-tree-error">{searchError}</p>;
        }

        if (searchResults.length === 0) {
            return (
                <div className="topic-browser-empty-tree">
                    <FolderTree size={26} />
                    <p>No topics found.</p>
                </div>
            );
        }

        return searchResults.map((topic) => {
            const canonicalTopic = topic?.topic || topic?.canonicalTopic || '';
            const displayPath = formatTopicPath(toTopicPathSegments(canonicalTopic));
            const selected = selectedTopic?.unsTreeId === topic?.unsTreeId || selectedTopic?.canonicalTopic === canonicalTopic;

            return (
                <button
                    key={topic?.unsTreeId || canonicalTopic}
                    type="button"
                    className={`topic-browser-search-result${selected ? ' active' : ''}`}
                    onClick={() => handleSearchResultSelect(topic)}
                >
                    <span>{topic?.name || displayPath || 'Topic'}</span>
                    <small>{displayPath}</small>
                </button>
            );
        });
    };

    const detailMetadataJson = useMemo(
        () => JSON.stringify(topicDetailRaw || topicDetail || selectedTopic?.topicRow || {}, null, 2),
        [topicDetailRaw, topicDetail, selectedTopic?.topicRow]
    );

    if (!deviceId) {
        return (
            <SelectDCD
                title="Select DCD"
                subtitle="Select the DCD whose data flows you want to browse."
                destinationPath="/dashboard/data-flows"
            />
        );
    }

    const showSearchResults = Boolean(searchTerm.trim());
    const detailSource = topicDetail || selectedTopic?.topicRow || {};
    const lastMessage = getLastMessageSummary(topicDetail);

    return (
        <div className="topic-browser-container">
            <div className="topic-browser-header">
                <div className="topic-browser-header-left">
                    <button
                        type="button"
                        className="topic-browser-back-btn"
                        onClick={() => router.push('/dashboard/data-flows')}
                        aria-label="Back to DCD selection"
                    >
                        <ArrowLeft size={22} />
                    </button>

                    <div>
                        <h1>Topic Browser</h1>
                        <p>Browse through the Unified Namespace and visualize your data with ease.</p>
                    </div>
                </div>

                <div className="topic-browser-device-pill">
                    <Server size={16} />
                    <span>{deviceName || 'Selected DCD'}</span>
                </div>
            </div>

            {catalogError && <div className="topic-browser-error-msg">{catalogError}</div>}

            {!catalogReady ? (
                <div className="topic-browser-ready-state">
                    <Database size={30} />
                    <h2>{isCatalogLoading ? 'Checking topic catalog...' : 'Device not yet ready'}</h2>
                    <p>
                        Waiting for the topic catalog to reach FULL_REPLACE before loading the browser.
                    </p>
                    {catalogStatus?.lastSyncMode && (
                        <span>Current sync mode: {catalogStatus.lastSyncMode}</span>
                    )}
                </div>
            ) : (
                <div className="topic-browser-grid">
                    <aside className="topic-browser-tree-card">
                        <div className="topic-browser-search">
                            <Search size={18} />
                            <input
                                type="search"
                                value={searchTerm}
                                onChange={(event) => setSearchTerm(event.target.value)}
                                placeholder="Search topics..."
                            />
                        </div>

                        <div className="topic-browser-tree-scroll">
                            {showSearchResults ? (
                                renderSearchResults()
                            ) : isTreeLoading ? (
                                <p className="topic-browser-tree-status">Loading topic tree...</p>
                            ) : treeError ? (
                                <p className="topic-browser-tree-error">{treeError}</p>
                            ) : rootNodes.length > 0 ? (
                                renderTreeNodes(rootNodes)
                            ) : (
                                <div className="topic-browser-empty-tree">
                                    <FolderTree size={26} />
                                    <p>No topics found.</p>
                                </div>
                            )}
                        </div>
                    </aside>

                    <main className="topic-browser-details-area">
                        {selectedTopic ? (
                            <>
                                <div className="topic-browser-topic-heading">
                                    <h2>Topic: {selectedTopicPath}</h2>
                                </div>

                                {detailError && <div className="topic-browser-detail-error">{detailError}</div>}

                                <section className="topic-browser-panel">
                                    <div className="topic-browser-panel-title">
                                        <ChevronDown size={18} />
                                        <h3>Topic Details</h3>
                                    </div>

                                    {isDetailLoading ? (
                                        <p className="topic-browser-panel-status">Loading topic details...</p>
                                    ) : (
                                        <div className="topic-browser-detail-grid">
                                            <DetailRow label="Location Path:" value={getTopicLocationPath(detailSource, selectedTopic)} />
                                            <DetailRow label="Data Contract:" value={getTopicDataContract(detailSource, selectedTopic)} />
                                            <DetailRow label="DCD:" value={deviceName || deviceId} />
                                            <DetailRow label="Tag Name:" value={getTagName(detailSource, selectedTopic)} />
                                        </div>
                                    )}
                                </section>

                                <section className="topic-browser-panel">
                                    <div className="topic-browser-panel-title standalone">
                                        <h3>Last Message</h3>
                                    </div>

                                    {isDetailLoading ? (
                                        <p className="topic-browser-panel-status">Loading last message...</p>
                                    ) : (
                                        <>
                                            <div className="topic-browser-message-grid">
                                                <MessageRow icon={FileText} label="Payload Type:" value={lastMessage.payloadType} />
                                                <MessageRow icon={Clock} label="Produced At:" value={lastMessage.producedAt} />
                                                <MessageRow icon={Database} label="Type:" value={lastMessage.type} />
                                                <MessageRow icon={Clock} label="Updated At:" value={lastMessage.updatedAt} />
                                            </div>

                                            <button
                                                type="button"
                                                className="topic-browser-metadata-toggle"
                                                onClick={() => setIsMetadataOpen((prev) => !prev)}
                                                aria-expanded={isMetadataOpen}
                                            >
                                                {isMetadataOpen ? <ChevronDown size={18} /> : <ChevronRight size={18} />}
                                                Metadata
                                            </button>

                                            {isMetadataOpen && (
                                                <div className="topic-browser-metadata-code">
                                                    <pre>{detailMetadataJson}</pre>
                                                </div>
                                            )}
                                        </>
                                    )}
                                </section>
                            </>
                        ) : (
                            <div className="topic-browser-start-state">
                                Start browsing the topic tree to view details.
                            </div>
                        )}
                    </main>
                </div>
            )}
        </div>
    );
};

export default TopicBrowser;
