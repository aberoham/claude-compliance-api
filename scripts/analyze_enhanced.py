#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "numpy>=1.24.0",
#     "pandas>=2.0.0",
#     "matplotlib>=3.7.0",
#     "seaborn>=0.12.0",
#     "scipy>=1.10.0",
#     "adjustText>=0.8",
# ]
# ///
"""
Enhanced Audit Logs Analyzer - Advanced visualizations and intelligent usage scoring for Claude audit logs.
Usage: uv run analyze_enhanced.py <path_to_csv_file> [options]

By default, only analyzes currently licensed users (fetched from Compliance API).
Use --all-users to include all users found in audit logs.

Examples:
  uv run analyze_enhanced.py audit_logs.csv --visualize
  uv run analyze_enhanced.py audit_logs.csv --period 30d --score-users
  uv run analyze_enhanced.py audit_logs.csv --all-users --visualize
  uv run analyze_enhanced.py audit_logs.csv --executive-summary --svg
"""

import sys
import csv
import json
import ast
import zipfile
import tempfile
import shutil
import re
import argparse
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.dates as mdates
import seaborn as sns
from datetime import datetime, timedelta
from collections import Counter, defaultdict
from pathlib import Path
from scipy import stats
from adjustText import adjust_text

# Configure matplotlib for better aesthetics
plt.style.use('seaborn-v0_8-darkgrid')
sns.set_palette("husl")

def parse_actor_info(actor_info_str):
    """Parse the actor_info field which uses Python dict literal syntax."""
    try:
        actor_data = ast.literal_eval(actor_info_str)
        return {
            'name': actor_data.get('name', 'Unknown'),
            'email': actor_data.get('metadata', {}).get('email_address', 'Unknown'),
            'uuid': actor_data.get('uuid', 'Unknown')
        }
    except (ValueError, SyntaxError):
        return {'name': 'Unknown', 'email': 'Unknown', 'uuid': 'Unknown'}

def parse_period(period_str):
    """Parse a period string like '7d' or '24h' into a timedelta."""
    match = re.match(r'^(\d+)([dhm])$', period_str.lower())
    if not match:
        raise ValueError(f"Invalid period format: {period_str}. Use format like '7d', '24h', or '30m'")
    
    value = int(match.group(1))
    unit = match.group(2)
    
    if unit == 'd':
        return timedelta(days=value)
    elif unit == 'h':
        return timedelta(hours=value)
    elif unit == 'm':
        return timedelta(minutes=value)
    else:
        raise ValueError(f"Unknown time unit: {unit}")

def calculate_usage_score(user_data, latest_timestamp, total_days):
    """
    Calculate intelligent usage score based on multiple factors.
    Returns score (0-100) and category.
    """
    # Parse timestamps
    last_seen = datetime.fromisoformat(user_data['last_seen'].replace('Z', '+00:00'))
    first_seen = datetime.fromisoformat(user_data['first_seen'].replace('Z', '+00:00'))
    latest = datetime.fromisoformat(latest_timestamp.replace('Z', '+00:00'))
    
    # Calculate metrics
    days_since_last_activity = (latest - last_seen).days
    days_active = (last_seen - first_seen).days + 1
    events_per_day = user_data['event_count'] / max(days_active, 1)
    
    # Recency score (0-30 points)
    if days_since_last_activity == 0:
        recency_score = 30
    elif days_since_last_activity <= 7:
        recency_score = 30 - (days_since_last_activity * 2)
    elif days_since_last_activity <= 30:
        recency_score = 16 - (days_since_last_activity - 7) * 0.5
    else:
        recency_score = max(0, 5 - (days_since_last_activity - 30) * 0.1)
    
    # Frequency score (0-30 points)
    if events_per_day >= 10:
        frequency_score = 30
    elif events_per_day >= 5:
        frequency_score = 25
    elif events_per_day >= 1:
        frequency_score = 20
    elif events_per_day >= 0.5:
        frequency_score = 15
    elif events_per_day >= 0.1:
        frequency_score = 10
    else:
        frequency_score = events_per_day * 100
    
    # Consistency score (0-20 points)
    if days_active >= 30:
        consistency_score = 20
    elif days_active >= 14:
        consistency_score = 15
    elif days_active >= 7:
        consistency_score = 10
    elif days_active >= 3:
        consistency_score = 5
    else:
        consistency_score = days_active
    
    # Diversity score (0-20 points)
    event_diversity = len(user_data['events'])
    platform_diversity = len(user_data['platforms'])
    diversity_score = min(20, (event_diversity * 2 + platform_diversity * 5))
    
    # Calculate total score
    total_score = recency_score + frequency_score + consistency_score + diversity_score
    
    # Determine category
    if total_score >= 70 and days_since_last_activity <= 7:
        category = "Power User"
    elif total_score >= 50 and days_since_last_activity <= 14:
        category = "Regular User"
    elif user_data['event_count'] <= 5 and days_active <= 2:
        category = "Exploratory User"
    elif days_since_last_activity > 30:
        category = "Dormant User"
    elif user_data['event_count'] == 1:
        category = "One-time User"
    else:
        category = "Occasional User"
    
    return {
        'score': round(total_score, 2),
        'category': category,
        'metrics': {
            'recency_score': round(recency_score, 2),
            'frequency_score': round(frequency_score, 2),
            'consistency_score': round(consistency_score, 2),
            'diversity_score': round(diversity_score, 2),
            'days_since_last_activity': days_since_last_activity,
            'days_active': days_active,
            'events_per_day': round(events_per_day, 2)
        }
    }

def analyze_audit_logs_enhanced(csv_file_path, period=None, active_users=None):
    """Analyze the audit logs CSV with enhanced data collection for visualizations."""
    user_activity = defaultdict(lambda: {
        'name': 'Unknown',
        'email': 'Unknown', 
        'event_count': 0,
        'events': Counter(),
        'platforms': Counter(),
        'first_seen': None,
        'last_seen': None,
        'daily_events': defaultdict(int),
        'hourly_events': defaultdict(int),
        'event_timestamps': []
    })
    
    # Track excluded users if filtering
    excluded_users = defaultdict(lambda: {
        'name': 'Unknown',
        'email': 'Unknown',
        'event_count': 0,
        'events': Counter(),
        'first_seen': None,
        'last_seen': None
    })
    
    # Time series data
    hourly_events = defaultdict(lambda: defaultdict(int))
    daily_events = defaultdict(lambda: defaultdict(int))
    event_timeline = []
    
    total_events = 0
    filtered_events = 0
    excluded_events = 0
    earliest_timestamp = None
    latest_timestamp = None
    
    # Calculate cutoff time if period is specified
    cutoff_time = None
    if period:
        with open(csv_file_path, 'r', encoding='utf-8') as file:
            reader = csv.DictReader(file)
            for row in reader:
                timestamp = row['created_at']
                if latest_timestamp is None or timestamp > latest_timestamp:
                    latest_timestamp = timestamp
        
        latest_dt = datetime.fromisoformat(latest_timestamp.replace('Z', '+00:00'))
        cutoff_time = latest_dt - period
    
    # Reset for actual processing
    latest_timestamp = None
    
    with open(csv_file_path, 'r', encoding='utf-8') as file:
        reader = csv.DictReader(file)
        
        for row in reader:
            total_events += 1
            timestamp = row['created_at']
            timestamp_dt = datetime.fromisoformat(timestamp.replace('Z', '+00:00'))
            
            if cutoff_time and timestamp_dt < cutoff_time:
                continue
                
            filtered_events += 1
            
            # Track global timespan
            if earliest_timestamp is None or timestamp < earliest_timestamp:
                earliest_timestamp = timestamp
            if latest_timestamp is None or timestamp > latest_timestamp:
                latest_timestamp = timestamp
                
            actor_info = parse_actor_info(row['actor_info'])
            uuid = actor_info['uuid']
            event_type = row['event']
            platform = row['client_platform']
            
            # Store event for timeline
            event_timeline.append({
                'timestamp': timestamp_dt,
                'user': uuid,
                'event': event_type,
                'platform': platform
            })
            
            # Aggregate time series data
            hour_key = timestamp_dt.strftime('%Y-%m-%d %H:00')
            day_key = timestamp_dt.strftime('%Y-%m-%d')
            
            hourly_events[hour_key][event_type] += 1
            daily_events[day_key][event_type] += 1
            
            if uuid != 'Unknown' and actor_info.get('email'):
                email = actor_info['email'].lower()
                
                # Check if user is active (if filtering is enabled)
                if active_users is not None and email not in active_users:
                    # Track excluded user
                    excluded_events += 1
                    excluded_user = excluded_users[uuid]
                    excluded_user['name'] = actor_info['name']
                    excluded_user['email'] = actor_info['email']
                    excluded_user['event_count'] += 1
                    excluded_user['events'][event_type] += 1
                    
                    if excluded_user['first_seen'] is None or timestamp < excluded_user['first_seen']:
                        excluded_user['first_seen'] = timestamp
                    if excluded_user['last_seen'] is None or timestamp > excluded_user['last_seen']:
                        excluded_user['last_seen'] = timestamp
                else:
                    # Process active user
                    user_data = user_activity[uuid]
                    user_data['name'] = actor_info['name']
                    user_data['email'] = actor_info['email']
                    user_data['event_count'] += 1
                    user_data['events'][event_type] += 1
                    user_data['platforms'][platform] += 1
                    user_data['daily_events'][day_key] += 1
                    user_data['hourly_events'][timestamp_dt.hour] += 1
                    user_data['event_timestamps'].append(timestamp_dt)
                    
                    if user_data['first_seen'] is None or timestamp < user_data['first_seen']:
                        user_data['first_seen'] = timestamp
                    if user_data['last_seen'] is None or timestamp > user_data['last_seen']:
                        user_data['last_seen'] = timestamp
    
    return {
        'user_activity': dict(user_activity),
        'filtered_events': filtered_events,
        'total_events': total_events,
        'earliest_timestamp': earliest_timestamp,
        'latest_timestamp': latest_timestamp,
        'hourly_events': dict(hourly_events),
        'daily_events': dict(daily_events),
        'event_timeline': event_timeline,
        'excluded_users': dict(excluded_users),
        'excluded_events': excluded_events
    }

def create_time_series_plot(data, output_dir, output_format='png'):
    """Create detailed time-series visualization."""
    fig, axes = plt.subplots(3, 1, figsize=(15, 12))
    
    # Convert daily events to DataFrame
    daily_df = pd.DataFrame.from_dict(data['daily_events'], orient='index')
    daily_df.index = pd.to_datetime(daily_df.index)
    daily_df = daily_df.sort_index()
    daily_df = daily_df.fillna(0)
    
    # Plot 1: Stacked area chart of event types
    ax1 = axes[0]
    daily_df.plot(kind='area', ax=ax1, alpha=0.7, stacked=True)
    ax1.set_title('Event Types Over Time (Stacked Area)', fontsize=14, fontweight='bold')
    ax1.set_xlabel('Date')
    ax1.set_ylabel('Number of Events')
    ax1.legend(title='Event Type', bbox_to_anchor=(1.05, 1), loc='upper left')
    
    # Plot 2: Total daily events with rolling average
    ax2 = axes[1]
    total_daily = daily_df.sum(axis=1)
    ax2.plot(total_daily.index, total_daily.values, alpha=0.5, label='Daily Events')
    
    # Add 7-day rolling average
    rolling_avg = total_daily.rolling(window=7, center=True).mean()
    ax2.plot(rolling_avg.index, rolling_avg.values, linewidth=2, label='7-day Rolling Average')
    
    ax2.set_title('Total Daily Events with Trend', fontsize=14, fontweight='bold')
    ax2.set_xlabel('Date')
    ax2.set_ylabel('Number of Events')
    ax2.legend()
    ax2.grid(True, alpha=0.3)
    
    # Plot 3: Hourly heatmap
    ax3 = axes[2]
    
    # Aggregate hourly data across all days
    hourly_pattern = defaultdict(lambda: defaultdict(int))
    for timestamp_str, events in data['hourly_events'].items():
        dt = datetime.strptime(timestamp_str, '%Y-%m-%d %H:00')
        weekday = dt.strftime('%A')
        hour = dt.hour
        for event_type, count in events.items():
            hourly_pattern[weekday][hour] += count
    
    # Create heatmap data
    weekdays = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday']
    hours = list(range(24))
    heatmap_data = []
    
    for day in weekdays:
        row = []
        for hour in hours:
            row.append(hourly_pattern[day][hour])
        heatmap_data.append(row)
    
    sns.heatmap(heatmap_data, ax=ax3, cmap='YlOrRd', xticklabels=hours, yticklabels=weekdays,
                cbar_kws={'label': 'Number of Events'})
    ax3.set_title('Activity Heatmap by Day of Week and Hour', fontsize=14, fontweight='bold')
    ax3.set_xlabel('Hour of Day')
    ax3.set_ylabel('Day of Week')
    
    plt.tight_layout()
    file_ext = 'svg' if output_format == 'svg' else 'png'
    output_path = Path(output_dir) / f'time_series_analysis.{file_ext}'
    if output_format == 'svg':
        plt.savefig(output_path, format='svg', bbox_inches='tight')
    else:
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
    plt.close()
    
    return output_path

def create_activity_scatterplot(data, output_dir, output_format='png'):
    """Create scatter plot showing user activity patterns with improved visibility."""
    fig, ax = plt.subplots(figsize=(16, 10))
    
    # Prepare user data with scores
    users_data = []
    latest_dt = datetime.fromisoformat(data['latest_timestamp'].replace('Z', '+00:00'))
    total_days = calculate_timespan_days(data['earliest_timestamp'], data['latest_timestamp'])
    
    for uuid, user_data in data['user_activity'].items():
        if user_data['event_count'] > 0:
            first_seen_dt = datetime.fromisoformat(user_data['first_seen'].replace('Z', '+00:00'))
            last_seen_dt = datetime.fromisoformat(user_data['last_seen'].replace('Z', '+00:00'))
            
            days_since_first = (latest_dt - first_seen_dt).days
            days_since_last = (latest_dt - last_seen_dt).days
            
            # Calculate usage score for color coding
            usage_score = calculate_usage_score(user_data, data['latest_timestamp'], total_days)
            
            users_data.append({
                'days_since_first': days_since_first,
                'event_count': user_data['event_count'],
                'days_since_last': days_since_last,
                'name': user_data['name'],
                'score': usage_score['score'],
                'category': usage_score['category']
            })
    
    if not users_data:
        # Create empty plot if no data
        ax.text(0.5, 0.5, 'No user data available', 
                horizontalalignment='center', verticalalignment='center',
                transform=ax.transAxes, fontsize=14)
        ax.set_xlabel('Days Since First Activity', fontsize=12)
        ax.set_ylabel('Total Events', fontsize=12)
        ax.set_title('User Activity Patterns', fontsize=14, fontweight='bold')
    else:
        users_df = pd.DataFrame(users_data)
        
        # Create scatter plot with one dot per user
        # Color by usage score (red to green)
        # Size by event count (larger = more events)
        sizes = 50 + (users_df['event_count'] / users_df['event_count'].max() * 200)
        
        scatter = ax.scatter(users_df['days_since_first'], 
                           users_df['event_count'],
                           c=users_df['score'], 
                           s=sizes,
                           cmap='RdYlGn',
                           alpha=0.7,
                           edgecolors='black',
                           linewidth=0.5,
                           vmin=0,
                           vmax=100)
        
        # Add colorbar
        cbar = plt.colorbar(scatter, ax=ax)
        cbar.set_label('Usage Score', fontsize=10)
        
        # Use log scale for y-axis
        ax.set_yscale('log')
        
        # Add labels with smart positioning
        texts = []
        for _, user in users_df.iterrows():
            # Truncate long names
            display_name = user['name'][:25] + '...' if len(user['name']) > 25 else user['name']
            
            # Create text object
            txt = ax.text(user['days_since_first'], 
                         user['event_count'],
                         display_name,
                         fontsize=7,
                         ha='center',
                         va='bottom')
            texts.append(txt)
        
        # Adjust text positions to avoid overlaps
        adjust_text(texts, 
                   x=users_df['days_since_first'].values,
                   y=users_df['event_count'].values,
                   arrowprops=dict(arrowstyle='-', color='gray', alpha=0.4, lw=0.5),
                   autoalign='y',
                   only_move={'points':'y', 'text':'y'},
                   force_points=0.1,
                   force_text=0.2)
        
        # Add category legend
        categories = users_df['category'].unique()
        category_colors = {'Power User': 'darkgreen', 
                         'Regular User': 'green',
                         'Occasional User': 'orange',
                         'Exploratory User': 'red',
                         'Dormant User': 'darkred',
                         'One-time User': 'maroon'}
        
        # Add legend for categories
        from matplotlib.patches import Patch
        legend_elements = []
        for cat in sorted(set(categories)):
            if cat in category_colors:
                legend_elements.append(Patch(facecolor=category_colors[cat], 
                                           label=f'{cat} ({len(users_df[users_df["category"]==cat])})',
                                           alpha=0.7))
        
        ax.legend(handles=legend_elements, title='User Categories', 
                 loc='upper left', frameon=True, fancybox=True, shadow=True)
        
        ax.set_xlabel('Days Since First Activity', fontsize=14)
        ax.set_ylabel('Total Events (log scale)', fontsize=14)
        ax.set_title('User Activity Patterns\nDot size = event count | Color = usage score (red=low, green=high)', 
                    fontsize=16, fontweight='bold')
        ax.grid(True, alpha=0.3)
        
        # Add some statistics
        ax.text(0.02, 0.98, f'Total Users: {len(users_df)}', 
               transform=ax.transAxes, fontsize=10, va='top',
               bbox=dict(boxstyle='round', facecolor='wheat', alpha=0.5))
    
    plt.tight_layout()
    file_ext = 'svg' if output_format == 'svg' else 'png'
    output_path = Path(output_dir) / f'activity_scatterplot.{file_ext}'
    if output_format == 'svg':
        plt.savefig(output_path, format='svg', bbox_inches='tight')
    else:
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
    plt.close()
    
    return output_path

def create_executive_summary_dashboard(data, scored_users, output_dir, output_format='png', active_users_count=None):
    """Create executive summary dashboard optimized for PowerPoint presentations."""
    # Set up professional styling
    plt.rcParams.update({
        'font.size': 12,
        'font.family': 'sans-serif',
        'axes.titlesize': 14,
        'axes.titleweight': 'bold',
        'figure.titlesize': 16,
        'figure.titleweight': 'bold'
    })
    
    # Define corporate color palette
    colors = {
        'primary': '#2E86C1',     # Blue
        'success': '#28B463',     # Green
        'warning': '#F39C12',     # Orange
        'danger': '#E74C3C',      # Red
        'info': '#8E44AD',        # Purple
        'neutral': '#566573'      # Gray
    }
    
    fig = plt.figure(figsize=(20, 12))
    
    # Main title
    fig.suptitle('Claude Enterprise Usage Analytics - Executive Summary', 
                fontsize=20, fontweight='bold', y=0.95)
    
    # Create 2x3 grid for 6 key metrics
    gs = fig.add_gridspec(2, 3, hspace=0.3, wspace=0.3)
    
    # 1. User Adoption Funnel (top left)
    ax1 = fig.add_subplot(gs[0, 0])
    
    total_users = len(data['user_activity'])
    category_counts = Counter(user['usage_score']['category'] for user in scored_users)
    
    power_users = category_counts.get('Power User', 0)
    regular_users = category_counts.get('Regular User', 0) 
    active_users_total = power_users + regular_users
    
    # Funnel data
    if active_users_count:
        funnel_data = [
            ('Licensed', active_users_count, colors['neutral']),
            ('Logged In', total_users, colors['info']),
            ('Active Users', active_users_total, colors['success']),
            ('Power Users', power_users, colors['primary'])
        ]
    else:
        funnel_data = [
            ('Total Users', total_users, colors['neutral']),
            ('Active Users', active_users_total, colors['success']),
            ('Power Users', power_users, colors['primary'])
        ]
    
    labels, values, bar_colors = zip(*funnel_data)
    y_pos = np.arange(len(labels))
    
    bars = ax1.barh(y_pos, values, color=bar_colors, alpha=0.8)
    ax1.set_yticks(y_pos)
    ax1.set_yticklabels(labels)
    ax1.set_xlabel('Number of Users')
    ax1.set_title('User Adoption Funnel')
    
    # Add value labels on bars
    for i, (bar, value) in enumerate(zip(bars, values)):
        ax1.text(bar.get_width() + max(values) * 0.01, bar.get_y() + bar.get_height()/2, 
                f'{value}', va='center', fontweight='bold')
    
    # 2. Engagement Trend (top center)
    ax2 = fig.add_subplot(gs[0, 1])
    
    # Convert daily events to trend
    daily_df = pd.DataFrame.from_dict(data['daily_events'], orient='index')
    if not daily_df.empty:
        daily_df.index = pd.to_datetime(daily_df.index)
        daily_df = daily_df.sort_index().fillna(0)
        total_daily = daily_df.sum(axis=1)
        
        # Show last 30 days if available
        if len(total_daily) > 30:
            total_daily = total_daily.tail(30)
        
        ax2.plot(total_daily.index, total_daily.values, 
                color=colors['primary'], linewidth=3, marker='o', markersize=4)
        
        # Add trend line
        x_numeric = np.arange(len(total_daily))
        z = np.polyfit(x_numeric, total_daily.values, 1)
        p = np.poly1d(z)
        ax2.plot(total_daily.index, p(x_numeric), 
                color=colors['warning'], linestyle='--', linewidth=2, alpha=0.8)
        
        ax2.set_title('Daily Activity Trend (Last 30 Days)')
        ax2.set_ylabel('Events per Day')
        ax2.tick_params(axis='x', rotation=45)
        
        # Growth indicator
        growth_pct = ((total_daily.iloc[-1] - total_daily.iloc[0]) / max(total_daily.iloc[0], 1)) * 100
        growth_color = colors['success'] if growth_pct > 0 else colors['danger']
        ax2.text(0.02, 0.98, f'Trend: {growth_pct:+.1f}%', 
                transform=ax2.transAxes, va='top', 
                bbox=dict(boxstyle='round', facecolor=growth_color, alpha=0.2))
    else:
        ax2.text(0.5, 0.5, 'Insufficient data for trend analysis', 
                ha='center', va='center', transform=ax2.transAxes)
        ax2.set_title('Daily Activity Trend')
    
    # 3. ROI Metrics (top right)
    ax3 = fig.add_subplot(gs[0, 2])
    
    # Calculate key ROI metrics
    total_events = sum(user['event_count'] for user in scored_users)
    avg_events_per_user = total_events / len(scored_users) if scored_users else 0
    avg_score = np.mean([user['usage_score']['score'] for user in scored_users]) if scored_users else 0
    
    # ROI indicators
    roi_metrics = [
        ('Avg Events/User', f'{avg_events_per_user:.1f}', colors['primary']),
        ('Avg Usage Score', f'{avg_score:.1f}/100', colors['success']),
        ('Active Rate', f'{(active_users_total/max(total_users,1)*100):.1f}%', colors['info'])
    ]
    
    if active_users_count:
        license_utilization = (total_users / active_users_count * 100) if active_users_count > 0 else 0
        roi_metrics.append(('License Utilization', f'{license_utilization:.1f}%', colors['warning']))
    
    # Create metric boxes
    for i, (metric, value, color) in enumerate(roi_metrics):
        y = 0.8 - (i * 0.2)
        ax3.text(0.1, y, metric, fontsize=12, fontweight='bold', transform=ax3.transAxes)
        ax3.text(0.9, y, value, fontsize=14, fontweight='bold', 
                color=color, ha='right', transform=ax3.transAxes)
    
    ax3.set_xlim(0, 1)
    ax3.set_ylim(0, 1)
    ax3.axis('off')
    ax3.set_title('Key Performance Indicators')
    
    # 4. User Categories Pie Chart (bottom left)
    ax4 = fig.add_subplot(gs[1, 0])
    
    category_data = list(category_counts.most_common())
    if category_data:
        labels, sizes = zip(*category_data)
        category_colors_map = {
            'Power User': colors['primary'],
            'Regular User': colors['success'], 
            'Occasional User': colors['warning'],
            'Exploratory User': colors['info'],
            'Dormant User': colors['danger'],
            'One-time User': colors['neutral']
        }
        pie_colors = [category_colors_map.get(label, colors['neutral']) for label in labels]
        
        wedges, texts, autotexts = ax4.pie(sizes, labels=labels, colors=pie_colors, autopct='%1.1f%%',
                                          startangle=90, textprops={'fontsize': 10})
        
        # Make percentage text bold
        for autotext in autotexts:
            autotext.set_color('white')
            autotext.set_fontweight('bold')
    
    ax4.set_title('User Category Distribution')
    
    # 5. Risk Analysis (bottom center)
    ax5 = fig.add_subplot(gs[1, 1])
    
    dormant_users = category_counts.get('Dormant User', 0)
    one_time_users = category_counts.get('One-time User', 0)
    at_risk = dormant_users + one_time_users
    
    risk_data = [
        ('Engaged', active_users_total, colors['success']),
        ('At Risk', at_risk, colors['danger']),
        ('Exploring', category_counts.get('Exploratory User', 0), colors['warning'])
    ]
    
    risk_labels, risk_values, risk_colors = zip(*risk_data)
    bars = ax5.bar(risk_labels, risk_values, color=risk_colors, alpha=0.8)
    
    # Add value labels
    for bar, value in zip(bars, risk_values):
        if value > 0:
            ax5.text(bar.get_x() + bar.get_width()/2, bar.get_height() + max(risk_values) * 0.01,
                    f'{value}', ha='center', va='bottom', fontweight='bold')
    
    ax5.set_title('User Engagement Risk Analysis')
    ax5.set_ylabel('Number of Users')
    
    # 6. Usage Timeline (bottom right)
    ax6 = fig.add_subplot(gs[1, 2])
    
    # Show weekly aggregation for better readability
    if not daily_df.empty:
        weekly_events = daily_df.resample('W').sum().sum(axis=1)
        
        if len(weekly_events) > 12:
            weekly_events = weekly_events.tail(12)
        
        bars = ax6.bar(range(len(weekly_events)), weekly_events.values, 
                      color=colors['primary'], alpha=0.7)
        
        # Week labels (simplified)
        week_labels = [f'W{i+1}' for i in range(len(weekly_events))]
        ax6.set_xticks(range(len(weekly_events)))
        ax6.set_xticklabels(week_labels, rotation=45)
        
        ax6.set_title('Weekly Usage Volume')
        ax6.set_ylabel('Total Events')
        
        # Highlight peak week
        max_idx = np.argmax(weekly_events.values)
        bars[max_idx].set_color(colors['warning'])
        bars[max_idx].set_alpha(1.0)
    else:
        ax6.text(0.5, 0.5, 'Insufficient data\nfor timeline', 
                ha='center', va='center', transform=ax6.transAxes)
        ax6.set_title('Weekly Usage Volume')
    
    # Add summary text box
    summary_text = f"""
    Analysis Period: {data['earliest_timestamp'][:10]} to {data['latest_timestamp'][:10]}
    Total Events: {data['filtered_events']:,}
    Peak Engagement: {max(total_daily.values) if not daily_df.empty else 'N/A'} events/day
    """
    
    fig.text(0.02, 0.02, summary_text.strip(), fontsize=10, 
             bbox=dict(boxstyle='round', facecolor='lightgray', alpha=0.5))
    
    # Save with appropriate format
    file_ext = 'svg' if output_format == 'svg' else 'png'
    output_path = Path(output_dir) / f'executive_summary_dashboard.{file_ext}'
    
    if output_format == 'svg':
        plt.savefig(output_path, format='svg', bbox_inches='tight', facecolor='white')
    else:
        plt.savefig(output_path, dpi=300, bbox_inches='tight', facecolor='white')
    
    plt.close()
    
    # Reset matplotlib settings
    plt.rcParams.update(plt.rcParamsDefault)
    
    return output_path

def create_usage_score_visualizations(scored_users, output_dir, output_format='png'):
    """Create visualizations for usage scores and categories."""
    fig, axes = plt.subplots(2, 2, figsize=(15, 12))
    
    # Prepare data
    categories = [user['usage_score']['category'] for user in scored_users]
    scores = [user['usage_score']['score'] for user in scored_users]
    
    # Plot 1: Category distribution
    ax1 = axes[0, 0]
    category_counts = pd.Series(categories).value_counts()
    category_counts.plot(kind='bar', ax=ax1, color='skyblue', edgecolor='black')
    ax1.set_title('User Categories Distribution', fontsize=14, fontweight='bold')
    ax1.set_xlabel('Category')
    ax1.set_ylabel('Number of Users')
    ax1.tick_params(axis='x', rotation=45)
    
    # Plot 2: Score distribution
    ax2 = axes[0, 1]
    ax2.hist(scores, bins=20, color='lightgreen', edgecolor='black', alpha=0.7)
    ax2.axvline(np.mean(scores), color='red', linestyle='--', label=f'Mean: {np.mean(scores):.1f}')
    ax2.set_title('Usage Score Distribution', fontsize=14, fontweight='bold')
    ax2.set_xlabel('Usage Score')
    ax2.set_ylabel('Number of Users')
    ax2.legend()
    
    # Plot 3: Score components breakdown for top users
    ax3 = axes[1, 0]
    top_users = sorted(scored_users, key=lambda x: x['usage_score']['score'], reverse=True)[:10]
    
    components = ['recency_score', 'frequency_score', 'consistency_score', 'diversity_score']
    component_data = {comp: [] for comp in components}
    user_names = []
    
    for user in top_users:
        user_names.append(user['name'][:20])  # Truncate long names
        for comp in components:
            component_data[comp].append(user['usage_score']['metrics'][comp])
    
    # Create stacked bar chart
    bottom = np.zeros(len(user_names))
    colors = ['#FF6B6B', '#4ECDC4', '#45B7D1', '#FFA07A']
    
    for i, comp in enumerate(components):
        ax3.bar(user_names, component_data[comp], bottom=bottom, 
               label=comp.replace('_', ' ').title(), color=colors[i])
        bottom += component_data[comp]
    
    ax3.set_title('Score Components for Top 10 Users', fontsize=14, fontweight='bold')
    ax3.set_xlabel('User')
    ax3.set_ylabel('Score')
    ax3.legend(loc='upper right')
    ax3.tick_params(axis='x', rotation=45)
    
    # Plot 4: Category vs Average Score
    ax4 = axes[1, 1]
    category_scores = defaultdict(list)
    for user in scored_users:
        category_scores[user['usage_score']['category']].append(user['usage_score']['score'])
    
    categories_list = []
    avg_scores = []
    std_scores = []
    
    for cat, scores_list in category_scores.items():
        categories_list.append(cat)
        avg_scores.append(np.mean(scores_list))
        std_scores.append(np.std(scores_list))
    
    x_pos = np.arange(len(categories_list))
    ax4.bar(x_pos, avg_scores, yerr=std_scores, capsize=5, 
           color='lightcoral', edgecolor='black', alpha=0.7)
    ax4.set_xticks(x_pos)
    ax4.set_xticklabels(categories_list, rotation=45)
    ax4.set_title('Average Score by Category', fontsize=14, fontweight='bold')
    ax4.set_xlabel('Category')
    ax4.set_ylabel('Average Score')
    
    plt.tight_layout()
    file_ext = 'svg' if output_format == 'svg' else 'png'
    output_path = Path(output_dir) / f'usage_scores_analysis.{file_ext}'
    if output_format == 'svg':
        plt.savefig(output_path, format='svg', bbox_inches='tight')
    else:
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
    plt.close()
    
    return output_path

def print_enhanced_summary(data, scored_users, csv_file_path, period=None, active_users_count=None, show_excluded=False, active_users=None):
    """Print enhanced summary with usage scores and categories."""
    timespan_days = calculate_timespan_days(data['earliest_timestamp'], data['latest_timestamp'])
    
    print(f"\n📊 Enhanced Audit Logs Analysis for: {Path(csv_file_path).name}")
    print("=" * 80)
    if period:
        print(f"Period Filter: Last {period}")
        print(f"Events in Period: {data['filtered_events']:,} of {data['total_events']:,} total ({(data['filtered_events']/data['total_events']*100):.1f}%)")
    else:
        print(f"Total Events: {data['filtered_events']:,}")
    
    # Show filtering stats if active users filtering was applied
    if active_users_count is not None:
        total_users_in_logs = len(data['user_activity']) + len(data['excluded_users'])
        print(f"\n🔐 Active User Filtering:")
        print(f"  Licensed users found in logs: {len(data['user_activity'])} of {active_users_count} total licenses")
        print(f"  Excluded (unlicensed) users: {len(data['excluded_users'])}")
        if data['filtered_events'] > 0:
            excluded_pct = (data['excluded_events']/(data['excluded_events'] + data['filtered_events'])*100)
        else:
            excluded_pct = 100.0
        print(f"  Excluded events: {data['excluded_events']:,} ({excluded_pct:.1f}% of total)")
    
    print(f"\nUnique Active Users: {len(data['user_activity'])}")
    print(f"Timespan: {data['earliest_timestamp'][:10]} to {data['latest_timestamp'][:10]} ({timespan_days} days)")
    print()
    
    # Category breakdown
    category_counts = Counter(user['usage_score']['category'] for user in scored_users)
    print("📈 User Categories Breakdown:")
    print("-" * 40)
    for category, count in category_counts.most_common():
        percentage = (count / len(scored_users)) * 100
        print(f"  {category}: {count} ({percentage:.1f}%)")
    print()
    
    # Top scored users
    top_scored = sorted(scored_users, key=lambda x: x['usage_score']['score'], reverse=True)[:70]
    print("🏆 Top 70 Users by Usage Score:")
    print("-" * 80)
    print(f"{'Rank':<5} {'Name':<30} {'Email':<30} {'Score':<8} {'Category':<15}")
    print("-" * 80)
    
    for i, user in enumerate(top_scored, 1):
        name = user['name'][:28] + '..' if len(user['name']) > 30 else user['name']
        email = user['email'][:28] + '..' if len(user['email']) > 30 else user['email']
        score = user['usage_score']['score']
        category = user['usage_score']['category']
        print(f"{i:<5} {name:<30} {email:<30} {score:<8.1f} {category:<15}")
    print()
    
    # Exploratory and Occasional Users
    exploratory = [u for u in scored_users if u['usage_score']['category'] == 'Exploratory User']
    occasional = [u for u in scored_users if u['usage_score']['category'] == 'Occasional User']
    
    if exploratory:
        print("🔍 Exploratory Users (testing phase):")
        print("-" * 60)
        for user in exploratory:
            metrics = user['usage_score']['metrics']
            print(f"  {user['name']} ({user['email']})")
            print(f"    Events: {user['event_count']}, Days active: {metrics['days_active']}")
            print(f"    Last seen: {metrics['days_since_last_activity']} days ago")
        print()
    
    if occasional:
        print("📅 Occasional Users:")
        print("-" * 60)
        for user in occasional:
            metrics = user['usage_score']['metrics']
            print(f"  {user['name']} ({user['email']})")
            print(f"    Events: {user['event_count']}, Days active: {metrics['days_active']}")
            print(f"    Events per day: {metrics['events_per_day']:.2f}")
            print(f"    Last seen: {metrics['days_since_last_activity']} days ago")
        print()
    
    # At-risk users (low scores or dormant)
    at_risk = [u for u in scored_users if u['usage_score']['category'] in ['Dormant User', 'One-time User']]
    if at_risk:
        print(f"⚠️  At-Risk Users ({len(at_risk)} total):")
        print("-" * 60)
        for user in at_risk[:10]:
            metrics = user['usage_score']['metrics']
            print(f"  {user['name']} ({user['email']})")
            print(f"    Category: {user['usage_score']['category']}")
            print(f"    Last seen: {metrics['days_since_last_activity']} days ago")
            print(f"    Total events: {user['event_count']}")
            print()
    
    # Show users with no audit logs if active users were provided
    if active_users_count is not None:
        # Detect users without logs
        active_users_emails = active_users if active_users else set()
        users_without_logs = detect_users_without_logs(active_users_emails, data['user_activity'])
        
        if users_without_logs:
            print(f"\n🔍 Active Users with NO Audit Log Entries ({len(users_without_logs)} users):")
            print("-" * 60)
            sorted_emails = sorted(users_without_logs)
            for i, email in enumerate(sorted_emails[:20], 1):
                print(f"  {i:2d}. {email}")
            if len(sorted_emails) > 20:
                print(f"  ... and {len(sorted_emails) - 20} more users")
            print("\n  These users likely have been invited but have not yet logged in or used the system.")
    
    # Show excluded users if requested
    if show_excluded and data.get('excluded_users'):
        print("\n🚫 Excluded Users (No Active License):")
        print("-" * 60)
        # Sort excluded users by event count
        excluded_sorted = sorted(data['excluded_users'].items(), 
                               key=lambda x: x[1]['event_count'], 
                               reverse=True)
        
        for uuid, user_data in excluded_sorted[:20]:  # Show top 20
            print(f"  {user_data['name']} ({user_data['email']})")
            print(f"    Events: {user_data['event_count']}")
            top_event = user_data['events'].most_common(1)[0] if user_data['events'] else ('N/A', 0)
            print(f"    Top event: {top_event[0]} ({top_event[1]}x)")
            print(f"    Active: {user_data['first_seen'][:10]} to {user_data['last_seen'][:10]}")
            print()
        
        if len(excluded_sorted) > 20:
            print(f"  ... and {len(excluded_sorted) - 20} more excluded users")

def calculate_timespan_days(start_timestamp, end_timestamp):
    """Calculate the number of days between two timestamps."""
    start = datetime.fromisoformat(start_timestamp.replace('Z', '+00:00'))
    end = datetime.fromisoformat(end_timestamp.replace('Z', '+00:00'))
    return (end - start).days + 1

def export_scored_users(scored_users, output_dir):
    """Export scored users to CSV."""
    output_path = Path(output_dir) / 'user_scores.csv'
    
    with open(output_path, 'w', newline='', encoding='utf-8') as csvfile:
        fieldnames = ['name', 'email', 'score', 'category', 'event_count', 
                     'days_since_last_activity', 'days_active', 'events_per_day',
                     'recency_score', 'frequency_score', 'consistency_score', 'diversity_score']
        writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
        
        writer.writeheader()
        for user in scored_users:
            row = {
                'name': user['name'],
                'email': user['email'],
                'score': user['usage_score']['score'],
                'category': user['usage_score']['category'],
                'event_count': user['event_count'],
                'days_since_last_activity': user['usage_score']['metrics']['days_since_last_activity'],
                'days_active': user['usage_score']['metrics']['days_active'],
                'events_per_day': user['usage_score']['metrics']['events_per_day'],
                'recency_score': user['usage_score']['metrics']['recency_score'],
                'frequency_score': user['usage_score']['metrics']['frequency_score'],
                'consistency_score': user['usage_score']['metrics']['consistency_score'],
                'diversity_score': user['usage_score']['metrics']['diversity_score']
            }
            writer.writerow(row)
    
    return output_path

def extract_zip_temporarily(zip_path):
    """Extract a zip file to a temporary directory and return the path to the CSV file."""
    temp_dir = tempfile.mkdtemp()
    try:
        with zipfile.ZipFile(zip_path, 'r') as zip_ref:
            zip_ref.extractall(temp_dir)
        
        csv_files = list(Path(temp_dir).rglob('*.csv'))
        if not csv_files:
            raise ValueError("No CSV files found in the zip archive")
        
        if len(csv_files) > 1:
            csv_files.sort(key=lambda x: x.stat().st_size, reverse=True)
        
        return str(csv_files[0]), temp_dir
    except Exception as e:
        shutil.rmtree(temp_dir, ignore_errors=True)
        raise e

def detect_users_without_logs(active_users_emails, audit_log_users):
    """
    Detect active users who have no entries in the audit logs.
    
    Args:
        active_users_emails: Set of email addresses from active users file
        audit_log_users: Dict of user data from audit logs
    
    Returns:
        Set of email addresses with no audit log entries
    """
    # Get all emails from audit logs (normalized to lowercase)
    audit_log_emails = set()
    for user_data in audit_log_users.values():
        email = user_data.get('email', '').lower()
        if email and email != 'unknown':
            audit_log_emails.add(email)
    
    # Find active users not in audit logs
    users_without_logs = active_users_emails - audit_log_emails
    
    return users_without_logs

def find_most_recent_file(path):
    """Find the most recent audit log file based on filename pattern."""
    path_obj = Path(path)
    
    if path_obj.is_file():
        return str(path_obj)
    
    if not path_obj.is_dir():
        raise ValueError(f"Path '{path}' is neither a file nor a directory")
    
    pattern = re.compile(r'audit_logs-(\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2})\.(csv|zip)$')
    
    candidates = []
    for file_path in path_obj.iterdir():
        if file_path.is_file():
            match = pattern.match(file_path.name)
            if match:
                timestamp = ''.join(match.groups()[:6])
                candidates.append((timestamp, file_path))
    
    csv_files = list(path_obj.glob('*.csv'))
    for csv_file in csv_files:
        if not pattern.match(csv_file.name):
            mtime = csv_file.stat().st_mtime
            candidates.append((str(mtime), csv_file))
    
    if not candidates:
        raise ValueError(f"No audit log files found in directory '{path}'")
    
    candidates.sort(key=lambda x: x[0], reverse=True)
    return str(candidates[0][1])

def main():
    parser = argparse.ArgumentParser(
        description="Enhanced Claude audit logs analyzer with visualizations and intelligent scoring.\n"
                    "By default, only analyzes currently licensed users (fetched from Compliance API).",
        epilog="Examples:\n"
               "  uv run analyze_audit_logs_enhanced.py audit_logs.csv --visualize\n"
               "  uv run analyze_audit_logs_enhanced.py audit_logs.csv --period 30d --score-users\n"
               "  uv run analyze_audit_logs_enhanced.py audit_logs/ --visualize --output-dir plots/\n"
               "  uv run analyze_audit_logs_enhanced.py audit_logs.csv --all-users --visualize\n"
               "  uv run analyze_audit_logs_enhanced.py audit_logs.csv --executive-summary --svg\n"
               "  uv run analyze_audit_logs_enhanced.py audit_logs.csv --visualize --executive-summary --svg",
        formatter_class=argparse.RawDescriptionHelpFormatter
    )

    parser.add_argument('path', help='Path to CSV file or directory containing audit logs')
    parser.add_argument('--period', type=str, help='Time period to analyze (e.g., 7d, 24h, 30m)')
    parser.add_argument('--visualize', action='store_true', help='Generate visualization plots')
    parser.add_argument('--score-users', action='store_true', help='Calculate and display user scores')
    parser.add_argument('--output-dir', type=str, default='audit_analysis_output',
                       help='Directory for output files (default: audit_analysis_output)')
    parser.add_argument('--all-users', action='store_true',
                       help='Analyze all users in logs, not just currently licensed users')
    parser.add_argument('--refresh-cache', action='store_true',
                       help='Force refresh of cached licensed user data from API')
    parser.add_argument('--show-excluded', action='store_true',
                       help='Show details of users in logs who are not currently licensed')
    parser.add_argument('--svg', action='store_true',
                       help='Output visualizations in SVG format instead of PNG')
    parser.add_argument('--executive-summary', action='store_true',
                       help='Generate executive summary dashboard for presentations')
    
    args = parser.parse_args()
    
    input_path = args.path
    
    if not Path(input_path).exists():
        print(f"Error: Path '{input_path}' not found.")
        sys.exit(1)
    
    # Create output directory if needed
    if args.visualize or args.score_users:
        output_dir = Path(args.output_dir)
        output_dir.mkdir(exist_ok=True)
        print(f"Output directory: {output_dir}")
    
    # Parse period if provided
    period = None
    period_str = None
    if args.period:
        try:
            period = parse_period(args.period)
            period_str = args.period
        except ValueError as e:
            print(f"Error: {e}")
            sys.exit(1)
    
    # Load licensed users from Compliance API (default behavior)
    active_users = None
    active_users_count = None
    if not args.all_users:
        try:
            from compliance_api import get_active_users_from_api
            print("Fetching licensed users from Compliance API...")
            active_users = get_active_users_from_api(use_cache=not args.refresh_cache)
            active_users_count = len(active_users)
            print(f"Loaded {active_users_count} licensed user emails from Compliance API")
        except Exception as e:
            print(f"Error fetching users from Compliance API: {e}")
            sys.exit(1)
    
    temp_dir = None
    try:
        # Find the most recent file if given a directory
        file_path = find_most_recent_file(input_path)
        print(f"Selected file: {Path(file_path).name}")
        
        # Handle zip files
        if file_path.endswith('.zip'):
            print("Extracting zip file...")
            csv_file_path, temp_dir = extract_zip_temporarily(file_path)
            print(f"Extracted CSV: {Path(csv_file_path).name}")
        else:
            csv_file_path = file_path
        
        # Analyze the CSV file
        print("Analyzing audit logs...")
        data = analyze_audit_logs_enhanced(csv_file_path, period, active_users)
        
        # Calculate usage scores
        scored_users = []
        total_days = calculate_timespan_days(data['earliest_timestamp'], data['latest_timestamp'])
        
        for uuid, user_data in data['user_activity'].items():
            usage_score = calculate_usage_score(user_data, data['latest_timestamp'], total_days)
            scored_users.append({
                'uuid': uuid,
                'name': user_data['name'],
                'email': user_data['email'],
                'event_count': user_data['event_count'],
                'usage_score': usage_score
            })
        
        # Print enhanced summary
        print_enhanced_summary(data, scored_users, csv_file_path, period_str, active_users_count, args.show_excluded, active_users)
        
        # Determine output format
        output_format = 'svg' if args.svg else 'png'
        
        # Generate visualizations if requested
        if args.visualize:
            print(f"\nGenerating visualizations ({output_format.upper()} format)...")
            
            # Time series plot
            ts_plot = create_time_series_plot(data, output_dir, output_format)
            print(f"  ✓ Time series analysis: {ts_plot}")
            
            # Activity scatter plot
            scatter_plot = create_activity_scatterplot(data, output_dir, output_format)
            print(f"  ✓ Activity patterns: {scatter_plot}")
            
            # Usage score visualizations
            if args.score_users:
                score_plot = create_usage_score_visualizations(scored_users, output_dir, output_format)
                print(f"  ✓ Usage score analysis: {score_plot}")
        
        # Generate executive summary dashboard if requested
        if args.executive_summary:
            print(f"\nGenerating executive summary dashboard ({output_format.upper()} format)...")
            exec_dashboard = create_executive_summary_dashboard(data, scored_users, output_dir, 
                                                              output_format, active_users_count)
            print(f"  ✓ Executive summary: {exec_dashboard}")
            print("\n📊 Executive dashboard ready for PowerPoint presentations!")
        
        # Export scored users if requested
        if args.score_users:
            csv_export = export_scored_users(scored_users, output_dir)
            print(f"\n📊 Exported user scores to: {csv_export}")
        
    except Exception as e:
        print(f"Error analyzing audit logs: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
    finally:
        if temp_dir:
            shutil.rmtree(temp_dir, ignore_errors=True)
            print("\nCleaned up temporary files.")

if __name__ == "__main__":
    main()
