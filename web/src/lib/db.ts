import { openDB as idbOpen, type IDBPDatabase, type DBSchema } from 'idb';

// ---------------------------------------------------------------------------
// IndexedDB gateway – single entry-point for all local persistence
// ---------------------------------------------------------------------------

const DB_NAME = 'mutqin-db';
const DB_VERSION = 1;

/** Schema definition for the Mutqin IndexedDB database. */
export interface MutqinDB extends DBSchema {
  students: {
    key: string;
    value: {
      id: string;
      name: string;
      createdAt: string;
      updatedAt: string;
      [key: string]: unknown;
    };
  };
  recitations_queue: {
    key: string;
    value: {
      id: string;
      studentId: string;
      createdAt: string;
      [key: string]: unknown;
    };
  };
  attendance_queue: {
    key: string;
    value: {
      id: string;
      studentId: string;
      date: string;
      [key: string]: unknown;
    };
  };
  quran_surahs: {
    key: number;
    value: {
      id: number;
      name: string;
      ayatCount: number;
      [key: string]: unknown;
    };
  };
  quran_ayat: {
    key: string;
    value: {
      id: string;
      surahId: number;
      ayahNumber: number;
      text: string;
      [key: string]: unknown;
    };
    indexes: { bySurah: number };
  };
  sync_meta: {
    key: string;
    value: {
      key: string;
      value: unknown;
      updatedAt: string;
    };
  };
}

/** Open (and create/upgrade) the Mutqin IndexedDB database. */
export function openDB(): Promise<IDBPDatabase<MutqinDB>> {
  return idbOpen<MutqinDB>(DB_NAME, DB_VERSION, {
    upgrade(db) {
      if (!db.objectStoreNames.contains('students')) {
        db.createObjectStore('students', { keyPath: 'id' });
      }
      if (!db.objectStoreNames.contains('recitations_queue')) {
        db.createObjectStore('recitations_queue', { keyPath: 'id' });
      }
      if (!db.objectStoreNames.contains('attendance_queue')) {
        db.createObjectStore('attendance_queue', { keyPath: 'id' });
      }
      if (!db.objectStoreNames.contains('quran_surahs')) {
        db.createObjectStore('quran_surahs', { keyPath: 'id' });
      }
      if (!db.objectStoreNames.contains('quran_ayat')) {
        const ayatStore = db.createObjectStore('quran_ayat', { keyPath: 'id' });
        ayatStore.createIndex('bySurah', 'surahId');
      }
      if (!db.objectStoreNames.contains('sync_meta')) {
        db.createObjectStore('sync_meta', { keyPath: 'key' });
      }
    },
  });
}
