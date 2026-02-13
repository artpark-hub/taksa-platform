import React from 'react';
import {
    Activity,
    HardDrive,
    Cpu,
    FileText,
    BarChart2,
    Terminal
} from 'lucide-react';
import './InstanceDetails.css';

const InstanceDetails = () => {
    return (
        <div className="instance-details-page">
            <div className="details-header">
                <div className="header-title">
                    <h1>Device Details</h1>
                    <p>Access and update the details of your devices.</p>
                </div>
            </div>

            <div className="details-columns-wrapper">

                {/* === COLUMN 1 (Agent & Message Queue) === */}
                <div className="details-column">
                    {/* Agent Card */}
                    <div className="detail-card">
                        <div className="card-header">
                            <h3>Agent <span className="status-dot green"></span></h3>
                        </div>
                        <div className="card-body">
                            <div className="input-group">
                                <label>Name</label>
                                <div className="text-box">AI-Impact-Summit-Demo</div>
                            </div>
                            <div className="input-group">
                                <label>Level 0</label>
                                <div className="text-box">Enterprise</div>
                            </div>
                            <div className="input-group">
                                <label>Level 1</label>
                                <div className="text-box">Delhi</div>
                            </div>
                            <div className="input-group">
                                <label>Level 2</label>
                                <div className="text-box">Office</div>
                            </div>
                            <div className="input-group">
                                <label>Level 3</label>
                                <div className="text-box text-muted">Your level 3 name</div>
                            </div>
                            <div className="input-group">
                                <label>Level 4</label>
                                <div className="text-box text-muted">Your level 4 name</div>
                            </div>

                            <div className="info-row right-align mt-2">
                                <label style={{ width: 'auto' }}>Latency</label>
                                <div className="value">N/A <span className="info-icon">ⓘ</span></div>
                            </div>
                        </div>
                        <div className="card-footer">
                            <button className="icon-btn"><FileText size={14} /> Logs</button>
                            <button className="icon-btn"><BarChart2 size={14} /> Metrics</button>
                        </div>
                    </div>

                    {/* Message Queue Card (Moved Here) */}
                    <div className="detail-card">
                        <div className="card-header">
                            <h3>Message Queue <span className="status-dot green"></span></h3>
                        </div>
                        <div className="card-body">
                            <div className="throughput-row">
                                <label>Incoming Throughput</label>
                                <span>795.2 B/s</span>
                            </div>
                            <div className="throughput-row">
                                <label>Outgoing Throughput</label>
                                <span>2.6 KiB/s</span>
                            </div>
                        </div>
                        <div className="card-footer">
                            <button className="icon-btn"><FileText size={14} /> Logs</button>
                            <button className="icon-btn"><BarChart2 size={14} /> Metrics</button>
                        </div>
                    </div>
                </div>

                {/* === COLUMN 2 (Container Only) === */}
                <div className="details-column">
                    {/* Container Card */}
                    <div className="detail-card-container">
                        <div className="card-header">
                            <h3>Container <span className="status-dot green"></span></h3>
                        </div>
                        <div className="card-body">
                            <div className="stat-row">
                                <label><Cpu size={14} /> CPU</label>
                                <div className="progress-wrapper">
                                    <div className="progress-text">3.9% of 8 Cores</div>
                                    <div className="progress-bar"><div className="fill" style={{ width: '3.9%' }}></div></div>
                                </div>
                            </div>
                            <div className="stat-row">
                                <label><Activity size={14} /> Memory</label>
                                <div className="progress-wrapper">
                                    <div className="progress-text">14.6% of 7.7 GiB</div>
                                    <div className="progress-bar"><div className="fill warning" style={{ width: '14.6%' }}></div></div>
                                </div>
                            </div>
                            <div className="stat-row">
                                <label><HardDrive size={14} /> Disk</label>
                                <div className="progress-wrapper">
                                    <div className="progress-text">61.8% of 460.4 GiB</div>
                                    <div className="progress-bar"><div className="fill danger" style={{ width: '61.8%' }}></div></div>
                                </div>
                            </div>

                            <div className="info-block mt-3">
                                <div className="info-row row-start mb-2">
                                    <label>Architecture</label>
                                    <span>arm64</span>
                                </div>
                                <div className="info-row multi-line">
                                    <label>Hardware ID</label>
                                    <span className="mono-text full-wrap">
                                        c92771048d29daf7762a3068a70a55a439bd2c1a10e4559d95f44b4d4f5441d6
                                    </span>
                                </div>
                            </div>
                        </div>
                        <div className="card-footer">
                            <button className="icon-btn"><Terminal size={14} /> S6 Logs</button>
                        </div>
                    </div>
                </div>

                {/* === COLUMN 3 (Data Flows & Release) === */}
                <div className="details-column">
                    {/* Data Flows Card */}
                    <div className="detail-card">
                        <div className="card-header">
                            <h3>Data Flows <span className="status-dot green"></span></h3>
                        </div>
                        <div className="card-body">
                            <div className="info-row row-start">
                                <label>Data Flows</label>
                                <span className="value-bold">1</span>
                            </div>
                            <div className="info-row row-start">
                                <label>Bridges</label>
                                <span className="value-bold">1</span>
                            </div>
                            <div className="info-row row-start">
                                <label>Status</label>
                                <span className="status-pill green-text">● Active: 1</span>
                            </div>
                        </div>
                    </div>

                    {/* Release Card */}
                    <div className="detail-card">
                        <div className="card-header">
                            <h3>Release <span className="status-dot green"></span></h3>
                        </div>
                        <div className="card-body">
                            <div className="info-row">
                                <label>Version</label>
                                <div className="value">v0.40.0</div>
                            </div>
                            <div className="info-row">
                                <label>Channel</label>
                                <div className="value">enterprise</div>
                            </div>
                            <table className="simple-table">
                                <thead>
                                    <tr>
                                        <th>Software</th>
                                        <th style={{ textAlign: 'right' }}>Version</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr><td>S6 Overlay</td><td style={{ textAlign: 'right' }}>3.2.0.2</td></tr>
                                    <tr><td>Benthos</td><td style={{ textAlign: 'right' }}>0.10.1</td></tr>
                                    <tr><td>Redpanda</td><td style={{ textAlign: 'right' }}>24.3.8</td></tr>
                                </tbody>
                            </table>
                        </div>
                    </div>
                </div>

            </div>
        </div>
    );
};

export default InstanceDetails;