import React, { useState, useEffect } from 'react';
import { ChevronRight, ChevronDown, Folder, Tag, MousePointerClick } from 'lucide-react';
import './Visualise.css';

const useLiveData = (initialData, pattern = 'random') => {
    const [data, setData] = useState(initialData);

    useEffect(() => {
        const interval = setInterval(() => {
            setData(prevData => {
                const lastPoint = prevData[prevData.length - 1];
                const lastValue = lastPoint.y;
                let newValue = lastValue;

                if (pattern === 'step') {
                    if (Math.random() > 0.9) {
                        newValue += (Math.random() > 0.5 ? 2000 : -2000);
                    }
                } else if (pattern === 'ramp') {
                    const drift = (Math.random() - 0.3) * 300;
                    newValue += drift;
                } else {
                    newValue += (Math.random() - 0.5) * 800;
                }

                if (newValue > 18000) newValue = 18000;
                if (newValue < 5000) newValue = 5000;

                const newPoint = {
                    id: Date.now(),
                    y: newValue,
                    timestamp: new Date().toLocaleTimeString()
                };

                return [...prevData.slice(1), newPoint];
            });
        }, 1000);

        return () => clearInterval(interval);
    }, [pattern]);

    return data;
};

// --- REUSABLE BASE CHART COMPONENT ---
const BaseDarkChart = ({ liveData, color = "#22d3ee" }) => {
    const width = 800;
    const height = 300;
    const maxY = 20000;
    const padding = 40;
    const [hoveredIndex, setHoveredIndex] = useState(null);

    const getY = (val) => height - padding - ((val / maxY) * (height - (padding * 2)));
    const getSvgX = (index) => index * (width / (liveData.length - 1));

    const pathData = liveData.map((p, i) =>
        `${i === 0 ? 'M' : 'L'} ${getSvgX(i)} ${getY(p.y)}`
    ).join(' ');

    const activeTooltip = hoveredIndex !== null ? {
        x: getSvgX(hoveredIndex),
        y: getY(liveData[hoveredIndex].y),
        value: Math.round(liveData[hoveredIndex].y),
        time: liveData[hoveredIndex].timestamp
    } : null;

    return (
        <div className="custom-dark-chart fade-in" onMouseLeave={() => setHoveredIndex(null)}>
            <svg viewBox={`0 0 ${width} ${height}`} className="chart-svg">
                {[5000, 10000, 15000].map(val => (
                    <g key={val}>
                        <line x1={padding} y1={getY(val)} x2={width} y2={getY(val)} stroke="#333" strokeWidth="1" strokeDasharray="4 4" />
                        <text x={padding - 10} y={getY(val) + 5} fill="#666" fontSize="10" textAnchor="end">{val.toLocaleString()}</text>
                    </g>
                ))}
                <text x={padding - 10} y={getY(20000) + 5} fill="#666" fontSize="10" textAnchor="end">20,000</text>
                <path d={pathData} fill="none" stroke={color} strokeWidth="2.5" strokeLinejoin="round" className="chart-line-path" />
                {liveData.map((p, i) => (
                    <circle
                        key={p.id || i}
                        cx={getSvgX(i)}
                        cy={getY(p.y)}
                        r={hoveredIndex === i ? 6 : 3}
                        fill="#000"
                        stroke={color}
                        strokeWidth="2"
                        className="chart-dot"
                        onMouseEnter={() => setHoveredIndex(i)}
                    />
                ))}
                {liveData.map((p, i) => (
                    i % 4 === 0 ? (
                        <text key={i} x={getSvgX(i)} y={height - 10} fill="#666" fontSize="10" textAnchor="middle">{p.timestamp}</text>
                    ) : null
                ))}
            </svg>
            {activeTooltip && (
                <div className="chart-tooltip" style={{ left: activeTooltip.x, top: activeTooltip.y - 15 }}>
                    <div className="tooltip-header">
                        <span className="tooltip-dot" style={{ background: color }}></span>
                        <span>Value</span>
                    </div>
                    <div className="tooltip-value">{activeTooltip.value}</div>
                    <div className="tooltip-time">{activeTooltip.time}</div>
                </div>
            )}
        </div>
    );
};

// --- CHART WRAPPER ---
const ChartWrapper = ({ color, pattern }) => {
    const generateInitial = () => {
        const arr = [];
        const now = new Date();
        let val = 10000;
        for (let i = 0; i < 20; i++) {
            val += (Math.random() - 0.5) * 500;
            arr.push({
                id: i,
                y: Math.max(5000, Math.min(18000, val)),
                timestamp: new Date(now.getTime() - ((20 - i) * 1000)).toLocaleTimeString()
            });
        }
        return arr;
    };
    const initialDataRef = React.useRef(generateInitial());
    const liveData = useLiveData(initialDataRef.current, pattern);
    return <BaseDarkChart liveData={liveData} color={color} />;
};

const Visualise = () => {
    const [selectedTag, setSelectedTag] = useState(null);

    // --- UPDATED: Start with empty state (All collapsed) ---
    // This forces the user to click 'artpark' to start drilling down.
    const [expandedNodes, setExpandedNodes] = useState({});

    const toggleNode = (nodeName) => {
        setExpandedNodes(prev => ({ ...prev, [nodeName]: !prev[nodeName] }));
    };

    const handleTagClick = (tagName) => {
        setSelectedTag(tagName);
    };

    // Helper component to render a folder node
    const FolderNode = ({ id, label, children }) => (
        <div className="tree-node">
            <div className={`node-label ${expandedNodes[id] ? 'active-parent' : ''}`} onClick={(e) => { e.stopPropagation(); toggleNode(id); }}>
                {expandedNodes[id] ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                <Folder size={16} className={`node-icon folder ${expandedNodes[id] ? 'open' : ''}`} />
                <span>{label}</span>
            </div>
            {/* Standard React conditional rendering for immediate open/close */}
            {expandedNodes[id] && (
                <div className="tree-children">
                    {children}
                </div>
            )}
        </div>
    );

    return (
        <div className="visualize-page">
            <div className="visualize-sidebar">
                <div className="sidebar-title">
                    <h3>Message Topic Explorer</h3>
                </div>

                <div className="tree-view">
                    {/* HIERARCHY: artpark > _historian > Root > Objects > DeviceSet > OPC UA > GlobalVars */}
                    <FolderNode id="artpark" label="ARTPARK">
                        <FolderNode id="historian" label="_HISTORIAN">
                            <FolderNode id="root" label="ROOT">
                                <FolderNode id="objects" label="OBJECTS">
                                    <FolderNode id="deviceset" label="DEVICE SET">
                                        <FolderNode id="opcua" label="OPC UA">
                                            <FolderNode id="globalvars" label="GLOBAL VARS">
                                                {/* TAGS */}
                                                {['OPC_TAG1', 'OPC_TAG2', 'OPC_TAG3'].map((tag) => (
                                                    <div
                                                        key={tag}
                                                        className={`tree-leaf ${selectedTag === tag ? 'selected' : ''}`}
                                                        onClick={() => handleTagClick(tag)}
                                                    >
                                                        <div style={{ display: 'flex', alignItems: 'center' }}>
                                                            <Tag size={14} className="node-icon tag" />
                                                            <span>{tag}</span>
                                                        </div>
                                                        {selectedTag === tag && <div className="active-dot"></div>}
                                                    </div>
                                                ))}
                                            </FolderNode>
                                        </FolderNode>
                                    </FolderNode>
                                </FolderNode>
                            </FolderNode>
                        </FolderNode>
                    </FolderNode>
                </div>
            </div>

            <div className="visualize-content">
                {selectedTag ? (
                    <div className="content-wrapper fade-in">
                        <div className="content-header">
                            <h2>{selectedTag}</h2>
                            <div className="tag-path">
                                ARTPARK / _HISTORIAN / ROOT / OBJECTS / DEVICESET / OPC UA / GLOBALVARS / <strong className="highlight">{selectedTag}</strong>
                            </div>
                        </div>

                        <div className="chart-container-dark">
                            <div className="live-tag">
                                <span className="pulse-dot"></span>
                                Live
                            </div>
                            {selectedTag === 'OPC_TAG1' && <ChartWrapper color="#22d3ee" pattern="random" />}
                            {selectedTag === 'OPC_TAG2' && <ChartWrapper color="#a78bfa" pattern="step" />}
                            {selectedTag === 'OPC_TAG3' && <ChartWrapper color="#34d399" pattern="ramp" />}
                        </div>
                    </div>
                ) : (
                    <div className="empty-state fade-in">
                        <div className="empty-icon-wrap">
                            <MousePointerClick size={32} color="#999" />
                        </div>
                        <p>Select a tag from the browser sidebar to visualize its real-time data.</p>
                    </div>
                )}
            </div>
        </div>
    );
};

export default Visualise;