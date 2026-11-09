import { request } from '../../utils/request';

Page({
  data: {
    keyword: '',
    dynasty: '全部',
    mood: '全部',
    author: '全部',
    tag: '全部',
    // 筛选项来自后端实时聚合（库中实际涉及的维度），默认仅「全部」占位。
    dynasties: ['全部'] as string[],
    moods: ['全部'] as string[],
    authors: ['全部'] as string[],
    tags: ['全部'] as string[],
    poems: [] as any[],
    loading: false,
  },

  // 每次进入/切回「诗库」tab：刷新筛选维度 + 列表，保证新发布的诗与新标签及时出现。
  onShow() {
    this.fetchFacets();
    const mood = wx.getStorageSync('lib_mood');
    if (mood) {
      wx.removeStorageSync('lib_mood');
      this.setData({ mood }, () => this.fetchList());
      return;
    }
    this.fetchList();
  },

  // 拉取库中实际涉及的朝代 / 心情 / 诗人 / 标签作为筛选项。
  fetchFacets() {
    request<any>({ url: '/poems/facets' })
      .then((f) => {
        this.setData({
          dynasties: f.dynasties || ['全部'],
          moods: f.moods || ['全部'],
          authors: f.authors || ['全部'],
          tags: f.tags || ['全部'],
        });
      })
      .catch(() => {});
  },

  fetchList() {
    this.setData({ loading: true });
    request<any[]>({
      url: '/poems',
      data: {
        q: this.data.keyword,
        dynasty: this.data.dynasty,
        mood: this.data.mood,
        author: this.data.author,
        tag: this.data.tag,
      },
    })
      .then((list) => this.setData({ poems: list || [], loading: false }))
      .catch(() => this.setData({ loading: false }));
  },

  onInput(e: any) {
    this.setData({ keyword: e.detail.value });
  },
  onSearch() {
    this.fetchList();
  },
  pickDynasty(e: any) {
    this.setData({ dynasty: e.currentTarget.dataset.v }, () => this.fetchList());
  },
  pickMood(e: any) {
    this.setData({ mood: e.currentTarget.dataset.v }, () => this.fetchList());
  },
  pickAuthor(e: any) {
    this.setData({ author: e.currentTarget.dataset.v }, () => this.fetchList());
  },
  pickTag(e: any) {
    this.setData({ tag: e.currentTarget.dataset.v }, () => this.fetchList());
  },

  goDetail(e: any) {
    const id = e.currentTarget.dataset.id;
    wx.navigateTo({ url: '/pages/poem/detail?id=' + id });
  },
});
