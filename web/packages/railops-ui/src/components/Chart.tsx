import ReactECharts from 'echarts-for-react';
import type { EChartsOption, EChartsReactProps } from 'echarts-for-react';
import type { CSSProperties } from 'react';
import { StatePanel } from './StatePanel';

export const railopsChartColors = ['#3B82F6', '#10B981', '#F59E0B', '#EF4444', '#64748B'];

export const railopsChartTheme = {
  color: railopsChartColors,
  animationDuration: 350,
  textStyle: { fontFamily: 'Inter, PingFang SC, Microsoft YaHei, sans-serif', color: '#64748B' },
  grid: { left: 42, right: 24, top: 32, bottom: 32, containLabel: true },
  tooltip: { trigger: 'axis', backgroundColor: '#1E293B', borderWidth: 0, textStyle: { color: '#FFFFFF' } },
  legend: { bottom: 0, icon: 'circle', itemWidth: 8, itemHeight: 8, textStyle: { color: '#64748B', fontSize: 12 } },
} as const;

export type ChartState = 'ready' | 'loading' | 'empty' | 'error';

export interface ChartContainerProps extends Pick<EChartsReactProps, 'notMerge' | 'lazyUpdate' | 'onEvents'> {
  option?: EChartsOption;
  state?: ChartState;
  height?: number | string;
  errorDescription?: string;
  onRetry?: () => void;
  className?: string;
  style?: CSSProperties;
}

export const ChartContainer = ({ option, state = 'ready', height = 320, errorDescription, onRetry, className, style, ...props }: ChartContainerProps) => {
  const chartStyle = { height, ...style };
  if (state === 'loading') return <div className={['railops-chart-container', className].filter(Boolean).join(' ')} style={chartStyle}><StatePanel state="loading" /></div>;
  if (state === 'empty') return <div className={['railops-chart-container', className].filter(Boolean).join(' ')} style={chartStyle}><StatePanel state="empty" /></div>;
  if (state === 'error') return <div className={['railops-chart-container', className].filter(Boolean).join(' ')} style={chartStyle}><StatePanel state="error" errorDescription={errorDescription} onRetry={onRetry} /></div>;
  return <div className={['railops-chart-container', className].filter(Boolean).join(' ')} style={chartStyle}><ReactECharts option={{ ...railopsChartTheme, ...option }} style={{ height: '100%', width: '100%' }} {...props} /></div>;
};
