import { request } from '../../utils/request';

Page({
  data: {
    dynasties: [] as any[],
    open: '',
    loading: true,
  },

  onShow() {
    this.load();
  },

  load() {
    const keep = this.data.open;
    const had = (this.data.dynasties || []).length > 0;
    request<any>({ url: '/history' })
      .then((res) => {
        const dynasties = res.dynasties || [];
        const still = dynasties.some((d: any) => d.name === keep);
        this.setData({
          dynasties,
          open: still ? keep : had ? keep : dynasties.length ? dynasties[0].name : '',
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
