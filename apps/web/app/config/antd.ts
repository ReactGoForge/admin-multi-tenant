import type { ConfigProviderProps } from 'antd'
import zhCN from 'antd/locale/zh_CN'

/** 统一维护 Ant Design 的语言环境、全局 token 和组件级主题配置。 */
export const antdConfig = {
  componentSize: 'small',
  locale: zhCN,
  theme: {
    token: {
      colorPrimary: '#3b6ff5',
      colorSuccess: '#10b981',
      colorWarning: '#f59e0b',
      colorError: '#ef4444',
      colorInfo: '#06a7c7',
      colorBgLayout: '#f4f7fb',
      colorBgContainer: '#ffffff',
      colorBgElevated: '#ffffff',
      colorBorder: '#e1e8f0',
      colorBorderSecondary: '#edf1f6',
      colorText: '#172033',
      colorTextSecondary: '#697386',
      borderRadius: 6,
      controlHeight: 36,
      fontSize: 14,
      fontWeightStrong: 600,
      fontFamily:
        'Inter, -apple-system, BlinkMacSystemFont, \'Segoe UI\', \'PingFang SC\', \'Microsoft YaHei\', sans-serif',
    },
    components: {
      Button: {
        borderRadius: 6,
        controlHeight: 36,
        defaultShadow: 'none',
        primaryShadow: 'none',
      },
      Card: {
        borderRadiusLG: 8,
        bodyPadding: 20,
        boxShadowTertiary: '0 8px 24px rgba(31, 45, 72, 0.06)',
        headerFontSize: 15,
        headerHeight: 48,
      },
      Dropdown: {
        borderRadiusLG: 8,
        paddingBlock: 6,
      },
      Form: {
        itemMarginBottom: 20,
        labelColor: '#344054',
        labelFontSize: 13,
      },
      Input: {
        activeBorderColor: '#3b6ff5',
        activeShadow: '0 0 0 3px rgba(59, 111, 245, 0.12)',
        hoverBorderColor: '#7b9cff',
        paddingInlineLG: 14,
      },
      Layout: {
        bodyBg: '#f4f7fb',
        headerBg: '#ffffff',
        headerHeight: 56,
        headerPadding: '0 20px',
        lightSiderBg: '#ffffff',
      },
      Menu: {
        groupTitleColor: '#98a2b3',
        groupTitleFontSize: 12,
        groupTitleLineHeight: '20px',
        iconMarginInlineEnd: 12,
        iconSize: 18,
        itemBorderRadius: 8,
        itemColor: '#475467',
        itemHeight: 40,
        itemHoverBg: '#f5f8fd',
        itemHoverColor: '#244fd3',
        itemMarginBlock: 2,
        itemMarginInline: 8,
        itemSelectedBg: '#edf3ff',
        itemSelectedColor: '#2f5ee5',
      },
      Table: {
        headerBg: '#f7f9fc',
        headerColor: '#172033',
      },
      Tabs: {
        cardBg: '#f7f9fc',
        cardGutter: 4,
        cardHeightSM: 38,
        cardPaddingSM: '7px 12px',
        itemActiveColor: '#244fd3',
        itemColor: '#667085',
        itemHoverColor: '#315fe8',
        itemSelectedColor: '#315fe8',
        titleFontSizeSM: 13,
      },
    },
  },
} satisfies Pick<ConfigProviderProps, 'componentSize' | 'locale' | 'theme'>
