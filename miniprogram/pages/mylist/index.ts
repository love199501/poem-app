import { request } from '../../utils/request';
import { loadCommentEnabled } from '../../utils/flags';
import { BASE_URL } from '../../config';

const TITLES: Record<string, string> = {
  favorites: '我的收藏',
  reads: '阅读历史',
  comments: '我的评论',
  likes: '获赞评论',
};

function toFull(u: string) {
  if (!u) return '';
  return /^https?:/.test(u) ? u : BASE_URL + u;
}
function fmtDate(s: string) {
  return (s || '').slice(0, 10);
}

Page({
  data: {
    type: 'favorites',
    title: '',
    poems: [] as any[],
    comments: [] as any[],
    loading: true,
    empty: false,
    commentEnabled: false,
  },

  onLoad(options: any) {
    const type = options.type || 'favorites';
    const title = TITLES[type] || '列表';
    wx.setNavigationBarTitle({ title });
    this.setData({ type, title });
    loadCommentEnabled().then((on) => {
      if (!on && (type === 'comments' || type === 'likes')) {
        this.setData({ commentEnabled: false, loading: false, empty: true });
        return;
      }
      this.setData({ commentEnabled: on });
      this.fetch();
    });
  },

  fetch() {
    const type = this.data.type;
    if (type === 'favorites' || type === 'reads') {
      const url = type === 'reads' ? '/users/me/history' : '/users/me/favorites';
      request<any[]>({ url })
        .then((list) => {
          const poems = (list || [])
            .filter((p) => String(p.firstVerse || '').trim())
            .map((p) => ({
              ...p,
              illust: toFull(p.illustrationUrl || ''),
              dateText: fmtDate(p.readAt),
            }));
          this.setData({ poems, loading: false, empty: poems.length === 0 });
        })
        .catch(() => this.setData({ loading: false, empty: true }));
    } else {
      // 评论与被赞均按时间倒排（最近在最上）。
      request<any[]>({ url: '/users/me/comments', data: { sort: 'new' } })
        .then((list) => {
          const comments = (list || []).map((c) => ({ ...c, dateText: fmtDate(c.createdAt) }));
          this.setData({ comments, loading: false, empty: comments.length === 0 });
        })
        .catch(() => this.setData({ loading: false, empty: true }));
    }
  },

  goDetail(e: any) {
    const id = e.currentTarget.dataset.id;
    if (id) wx.navigateTo({ url: '/pages/poem/detail?id=' + id });
  },
});
