import { request } from '../../utils/request';

Page({
  data: {
    dynasties: [] as any[],
    open: '',
    loading: true,
  },

  onShow() {
    this.setData({ open: '' });
    this.load();
  },

  load() {
    request<any>({ url: '/history' })
      .then((res) => {
        this.setData({
          dynasties: res.dynasties || [],
          open: '',
          loading: false,
        });
      })
      .catch(() => this.setData({ loading: false }));
  },

  toggle(e: any) {
    const name = e.currentTarget.dataset.name;
    this.setData({ open: this.data.open === name ? '' : name });
  },

  goPoet(e: any) {
    const author = e.currentTarget.dataset.author;
    const dynasty = e.currentTarget.dataset.dynasty || '';
    if (!author) return;
    wx.navigateTo({
      url: '/pages/poet/scroll?author=' + encodeURIComponent(author) + '&dynasty=' + encodeURIComponent(dynasty),
    });
  },
});
