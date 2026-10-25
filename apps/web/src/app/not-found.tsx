import React from 'react';
import type { Metadata } from 'next';
import { NotFoundContent } from '@/components/NotFoundContent';

export const metadata: Metadata = {
  title: 'Halaman tidak ditemukan | Sela',
};

export default function NotFound() {
  return <NotFoundContent />;
}
