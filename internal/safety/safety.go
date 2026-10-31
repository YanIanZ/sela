// Package safety is Sela's deterministic safety router. It runs before every
// model call and before every ordinary handoff transition (SPEC V3).
//
// It is a rule-based lexicon for Indonesian, English and code-switched text,
// ported from the inherited Dengar.in crisis engine
// (services/crisis-engine/src/crisisDetector.ts). It is not a clinical risk
// assessment and makes no safety promise: it decides which product path is
// shown. Ambiguous input takes the safer path.
//
// The package has no network, model or logging dependency. Assessment values
// carry rule identifiers only, never the input text, so callers may log them.
//
// # Matching
//
// Assess normalises the text (see Normalize) and matches the rule catalogue
// against the canonical form and up to six derived variants (see variants):
// dash-joined, double-letter, a free-standing "1" read as "i", a lexical
// reading (every elongated word resolved against the catalogue vocabulary,
// two-letter elongation inside a word included; fused spaced-out words
// split; symbols, emoji, emphasis marks and spaced dashes between words read
// as spaces, and hesitation dots, or a pause before a newline, as a newline
// that keeps the canonical form's clause boundary; contractions, common
// misspellings and chat spellings spelt out: "aq", "ak" and "sy" as
// pronouns, "aj" as "aja", "bsk" as "besok", "tdk" as "tidak", "tonite" as
// "tonight", and a free-standing "w" as "gw" next to an Indonesian word or
// as "with" before an English object), its dash-joined form, and an alternative
// leetspeak reading (@ $ ! | 7 +, and each 1 or | read as i or l, whichever
// spells a vocabulary word: "k1ll", "$u1c1d3", "m@t1"; a final "!" after a
// whole word stays punctuation: "m@ti! Lampu kamar mati"). Variants only
// ever add matches: each is matched in its own context and the strongest
// outcome per rule is kept. Adoption (see Routing policy) is folded over all
// variants, so "my friend said she wants to die. me tooooo" counts although
// only one variant spells "me too". Every v1 rule is ported with its v1 id;
// v1 severity 'crisis' maps to LevelCrisis and 'high' to LevelElevated. Go's
// regexp is RE2 and has no lookarounds; the two v1 negative lookaheads
// (passive-ideation and en-end-everything), the idiom checks and the context
// markers are written as post-match functions documented on each rule (most
// of them in checks.go). Rules run per sentence ('.', '!', '?'), which finds
// exactly what a whole-text search finds, behind a literal prefilter derived
// from each expression. The checks of a wide pattern also read up to
// restWindow bytes of the text after its sentence, for an elliptical
// continuation ("I hope I don't wake up late. or ever") and for a no-return
// tail far from a brief-farewell phrase.
//
// Assess runs in time linear in the length of the input. A long sentence
// (768 bytes or more, such as an unpunctuated voice transcript) is matched
// only in windows around the literal atoms of each pattern, bounded by the
// longest match the pattern allows (see window.go). An adversarial
// 8000-character text takes up to about 30ms and a realistic one about 5ms
// in a normal build; a 32000-byte unpunctuated transcript made of the benign
// vectors takes about 0.25s and 48000 bytes of a bare slang word repeated
// about 10ms (TestPerformance bounds the 8000-character inputs at 200ms and
// the 32000-byte transcript at 800ms; TestLinearTime
// checks the growth of every adversarial unit, under the race detector
// too). It has no input cap of its own, because truncating would drop risk
// language; callers bound request sizes.
//
// # Routing policy
//
// Matches are then judged in context. Ambiguity takes the safer path, and a
// false-positive fix never costs a first-person false negative.
//
//  1. Clauses. The text is split into clauses at . ! ? ; and newlines and at
//     the conjunctions tapi, tetapi, tp, tpi, namun, padahal, melainkan,
//     sedangkan, but, however, although and whereas (never inside quotation
//     marks), and clauses into segments at commas. Context (denial, third
//     party) is decided per segment or clause, while matching also sees the
//     whole text, so a phrase that spans clauses ("aku mau\nmati") counts.
//     The anchored v1 pattern "^kill me$" is tried on the whole text and on
//     each clause.
//  2. Denial. A segment that is an explicit whole-clause denial in v1's
//     contextualDiscussion shape ("aku tidak mau bunuh diri", "I don't want
//     to kill myself"; widened to colloquial pronouns and negators, a lead-in
//     word such as "honestly" or "jujur", trailing particles such as "kok",
//     and "I'm not suicidal") does not count, and Denial is set. Only that
//     segment is exempt: "Aku tidak mau bunuh diri tapi aku ingin mengakhiri
//     hidupku malam ini" is crisis, and passive ideation after a denial ("I'm
//     not suicidal, just tired of living") is elevated. Without the comma a
//     denial that "just" or "cuma" continues ends before it only when a
//     listed tiredness, sadness or passive-ideation phrase follows ("I'm not
//     suicidal just tired of living") or a wish for a listed benign thing
//     ("just need a break", "cuma pengen istirahat", "just want someone to
//     talk to"); any other continuation, any other wish included ("just
//     want to not exist", "cuma pengen nggak ada"), leaves the segment the
//     speaker's own words, as in v1. So does a listed phrase after which
//     the continuation or the rest of its sentence names a self-directed
//     method ("just need a break with my pills", "cuma pengen tidur, pakai
//     semua obatku"), a deferral to the act or a threat ("... before I end
//     it", "... or I'll do something", "... atau aku bakal nekat"), a later
//     act, an absence or an ending in a later segment ("..., permanently")
//     or another wish ("..., i want out"). What "just"/"cuma" adds, after a
//     comma or opening the next clause too, can take the denial back: a
//     retraction, a deferral, waiting or planning, a condition, a prepared
//     means, a lack of courage or a wish for an absence or an ending ("I
//     don't want to kill myself, just yet", "just kidding", "just not
//     today", "just waiting for the right time", "only because of my mom",
//     "cuma untuk sekarang", "cuma udah siapin obat", "cuma belum berani",
//     "just too scared to do it", "just want to be gone") makes the risk
//     phrase count, and thinking about it, fear, a negation or any other
//     wish ("cuma kadang kepikiran", "just scared", "just want the pain to
//     stop") hedges it; the rest of the sentence after it is read the same
//     way (a method, a deferral, a threat, a later act or an absence takes
//     the denial back, and another wish hedges it). A retraction or a
//     deferral that opens the next segment or clause without "just"/"cuma",
//     after laughter or a filler, takes the denial back too ("I'm not
//     suicidal, jk", "..., haha just kidding", "..., not really", "aku gak
//     mau bunuh diri, boong deng",
//     "..., gak deng", "..., belum"), while laughter or reassurance alone
//     ("..., haha", "..., not at all", "..., I promise") leaves it. A later
//     act in the next segment, clause or sentence takes the denial back
//     too: a first-person future or progressive "do it" with a time or
//     sincerity word ("..., I promise I'll do it tonight", "..., serius
//     besok aku lakuin"), a prepared means ("..., I've got the pills
//     ready", "..., udah siapin obat") or a set time ("..., lol tonight is
//     the night"). A denial that is
//     followed by a contrastive conjunction, after its "just"/"cuma"
//     continuation too, is hedged and counts as elevated, whether or not
//     anything follows the conjunction ("aku tidak mau bunuh diri, tapi kadang
//     kepikiran", "I don't want to die, but...", "I'm not suicidal just
//     tired, but"). As in v1 the whole text is also checked as one denial, so
//     the words of a denial may be separated by newlines.
//  3. Third party. Risk language clearly attributed to someone else, with no
//     first-person risk, routes to LevelElevated with ThirdParty set (see
//     Third-party attribution). v1 returned none for its media frames; this is
//     an intentional change, so concern about someone else still surfaces
//     human-support information. When the speaker adopts the language anywhere,
//     each attributed match counts in full: "me too", "metoo", "me 2", "so do
//     I", "same here", "omg same", "lowkey same", "literally me", "this is me",
//     "big mood", "felt", "I relate", "aku relate", "i feel that", "i do, too",
//     "i've been feeling the same", "i think about it too", "sama kayak aku",
//     "kayak aku", "sama aja aku", "gw banget", "i know the feeling", "i know
//     how she feels", "been there", "same boat", "join the club", "ditto", "it
//     me", "I want that", "aku tau rasanya", "senasib", "kita sama" (these
//     before trailing words only); "aku juga", "gw jg", "aku pun",
//     "saya juga" and the chat spellings "aq juga", "ak juga", "w juga", "sy
//     juga", unless an ordinary continuation follows right after them ("aku
//     juga nonton beritanya", "aku pun kaget"): after an aspect word ("udah",
//     "lagi", "kadang") or a negator or limiter ("nggak", "cuma", "ada",
//     "punya", "baru") only a knowing or perceiving verb is ordinary ("aku juga
//     udah tahu", "aku juga nggak tahu"), a wish, thought, intent, hopelessness
//     or death word is never ordinary ("aku juga udah nggak sanggup", "aku juga
//     punya rencana"), and any other word takes the safer path ("aku juga dari
//     dulu"); "aku sama", "so am I", "I'm the same", "same" or "sama" opening
//     its words, or an elliptical wish ("aku pengen", "I want to") followed by
//     nothing but laughter, emoticons, emoji, particles or time and degree
//     adverbs, chat fillers included ("idk", "ugh", "omg", "af", "anjir",
//     "asli": "aku juga wkwk", "so am I :(", "aku pengen banget", "same
//     idk"), where
//     laughter, an emoticon, a dash or a time adverb closes any form but a wish
//     ("so am i lol idk what to do"); a wish or thought that refers back to the
//     act ("jadi pengen ngelakuin juga", "pengen banget ngelakuin itu",
//     "kepikiran buat ngelakuin hal yg sama", "I'm thinking of doing the
//     same", "I want to too"); an imitation
//     ("pengen nyusul", "aku mau ikut", "aku pengen kayak dia", "kok aku iri",
//     "I want to try it"); a wish or envy the speaker attaches to the reported
//     act, or the speaker next in line ("I wish that was me", "andai itu aku",
//     "I wish I had the guts", "kenapa bukan aku", "why not me", "itu yang aku
//     mau", "that's what I want", "I'm next", "semoga aku berikutnya");
//     "juga"/"too" inside the reported words; or "too" after third-person wish
//     or intent wording ("she wants to die too"; not after a past act: "my
//     uncle killed himself too" stays elevated). A bare third-person subject
//     without a frame ("temanku mau bunuh diri") is not attributed and stays
//     crisis. A hope for the speaker's death that a third-party subject voices
//     ("they wish I would die", "mereka berharap aku mati") is attributed when
//     the third party opens its clause or follows a clause word ("terus
//     mereka berharap aku mati"); after a verb or a preposition it is an
//     object, and the hope is the speaker's own run-on clause ("aku kangen
//     ibu berharap aku meninggal", "i hate you wish i would die" are crisis).
//  4. Study or prevention context is not recognised: "aku sedang belajar
//     tentang pencegahan bunuh diri" stays crisis exactly as in v1. This is a
//     known, deliberate conservative false positive.
//  5. Everyday idioms and benign uses stay none, but only when no intent, plan
//     or self-directed method is expressed. Where a rule matches one, a
//     documented post-check rejects it. A fixed idiom is a set phrase that
//     completes the match and has no self-harm reading: a device, power, plant
//     or animal subject ("baterai hp-ku mau mati", "kucingku mau mati"; only an
//     animal for a jump: "kucingku loncat dari atap"), an idiom noun after
//     "mati" ("mau mati lampu", "mati gaya", "mati rasa" at the end of its
//     phrase), "mati-matian", "bunuh diri"/"suicide" inside a compound ("gol
//     bunuh diri", "career suicide", "Suicide Squad"; a news compound such as
//     "bom bunuh diri" is attributed), "burning myself out", "cut myself off
//     from", "shooting myself in the foot", "drown myself in work", "cutting
//     carbs", "cutting back on", "die" spelt for "dye" ("I need to die my hair
//     black"), vanishing forever from social media or someone's life, a
//     relative's will ("kakek bikin surat wasiat"), a hope not to wake up that
//     names a routine as a whole phrase ("I hope I don't wake up late", "...
//     too early", "... before my alarm", "... with a headache", "... before
//     noon on weekends"; not "at all", a final "too", "again", "sick" or "with
//     this pain", not "by" a time or a year, and not with an intent or death
//     word near it, in the sentence or in the text after it: "... late. or
//     ever"), English compounds such as "die cast", and the completions of the
//     hope, wish and overdose forms: dying of old age, growing old and dying,
//     "die of happiness", "dead wrong", "dead asleep", "dead honest", "not
//     alive to see it" ("it", "that", "what" or "how"; not a time, "the" or
//     "this": "... to see the morning" is crisis), a death in a game that
//     happens in the game noun phrase right after the match ("I hope I die
//     first in the game", "... make it to the next round" with a game word
//     in the sentence; not after a conjunction or "before", "this game
//     called life" or "the game of life"), not seeing the sunrise while
//     sleeping in, a listed everyday destination or occasion after "make
//     it" ("... make it to the meeting", "... in time", "... for dinner";
//     not "at all", "home", "there", "in this world", "into tomorrow", a
//     day or a time), being away for a listed everyday occasion, trip or
//     place after a hope or a wish to be gone ("I hope I'm gone before
//     traffic gets bad", "I hope I'm not here when the movers come", "I want
//     to be gone by the time the party starts"), an online absence or a
//     stay elsewhere after a wish not to exist or not to be here ("I want to
//     not exist on social media", "I want to not be here today"), "nggak
//     ada" before a listed object or an everyday place ("pengen nggak ada
//     pr", "semoga aku nggak ada di kelas besok"; not "di muka bumi", "di
//     alam ini", "di tahun depan", "di sini lagi" or "di antara kalian"),
//     leaving a place or ending something with someone after a wish to go
//     forever or be done ("pengen pergi selamanya dari kota ini", "pengen
//     udahan aja sama dia"), a way of living after a wish to stop living
//     ("pengen berhenti hidup boros", "... hidup dengan boros", "... tanpa
//     tujuan"; not a manner of dying or a method: "... dengan tenang",
//     "... tanpa rasa sakit", "... dengan minum obat"), a short break after
//     a wish to leave this world ("pengen pergi dari dunia ini sebentar";
//     not the farewell "dulu", "sebentar lagi" or "selamanya" later in the
//     sentence), the legacy idiom after "leave this world" ("a better
//     place", "a little better than I found it"; not "a little early", "a
//     bit sooner", "better off" or "without me" later in the sentence),
//     devotion ("aku siap mati demi kamu"), a life that ends happily, "die
//     on this hill", a past bottle of something else ("I took the whole
//     bottle of water"), "my heart stops racing", a plea
//     to be taken to an everyday destination, a trip or a past year ("God
//     take me back to school", "lord take me higher", "God take me to
//     Bali", "take me back to 2019"; not "take me home", "to heaven", "to
//     my family", "out of this world", "there" or "somewhere" to rest, and
//     not with a self-directed method or an intent word anywhere in the
//     sentence: "God take me to the beach and let me drown"), a subjectless
//     curse on someone else whose victim ends the clause ("semoga cepet
//     mati tuh orang", "moga mampus si koruptor"; not "semoga cepet mati
//     semuanya beres" or "... itu aja"), a question about taking medicine
//     together ("boleh minum obat sekaligus dengan vitamin?"; not with a
//     quantity after it: "... sekaligus semuanya") and
//     "menenggelamkan diri dalam pekerjaan"; a risk word anywhere in the
//     sentence keeps a hope or wish idiom. An overdose plan is never
//     narrowed by its object. A negated intent or hope ("I don't want to take
//     all my pills", "I hope I don't die") is none unless a contrastive
//     conjunction hedges it, in its sentence or opening the next one ("... my
//     pills. but sometimes I think about it" is crisis). A
//     compound that an intent word governs is the speaker's own plan and counts
//     ("aku mau jalanin misi bunuh diri", "aku mau bom bunuh diri", "I want to
//     be a suicide bomber"), and so is a news compound or "bunuh diri karier"
//     with a first-person subject ("aku pelaku bom bunuh diri", "aku bunuh diri
//     karier"); "siap", "rela" and "bersedia" govern a compound only for the
//     speaker or a subjectless clause ("teroris itu siap jadi pelaku bom
//     bunuh diri" is attributed, and a readiness attributed this way never
//     makes a compound an idiom: "teroris itu siap jalanin misi bunuh diri" is
//     attributed too), while a third party that is part of a subject with the
//     speaker or follows a preposition or a verb ("aku dan temanku siap ...",
//     "demi mereka siap ...", "aku ikut mereka siap ...", a comma after the
//     preposition read as a slip: "demi, mereka siap ...") or a first-person
//     pronoun earlier in the clause leaves the readiness the speaker's own;
//     "I'm on a suicide mission at work" stays an idiom. "kms" after a number
//     or a quantity word ("5 kms", "a few kms", "I'll run until 10 kms" after
//     a motion verb, "5 kms per hour"; "at 8 kms", "by 9 kms", "jam 12 kms
//     lagi" and "wait until 10 kms" are a time: after a clock word only
//     "per", "away" or another distance word makes a speed or a distance)
//     or as the child health card ("KMS anakku", near "posyandu") is not the
//     slang, unless an intent word governs it ("i want to kms"; "aku mau lari 5
//     kms" is a distance). A leisure jump by a group ("kami terjun dari tebing
//     di bali seru banget") is rejected when its phrase has no intent word. A
//     context marker is a word right next to the match that suggests a benign
//     reading: an accident told as one past event ("I cut myself shaving", "I
//     hurt myself at the gym", "I burned myself on the stove", "I hit myself
//     on the cabinet door": furniture or a door, while an instrument or any
//     other surface needs an accident word, "I hit myself with a hammer by
//     accident"; "nggak sengaja ngiris tangan pas potong bawang"),
//     medication taken as prescribed ("I took all my pills like the doctor
//     said"; a present or future plan to take them has no marker) and a legal
//     will ("cara bikin surat wasiat yang sah"). The gate for a context marker
//     is closed by default, and it is open only when it is open on every
//     variant of the text and the original text, before normalisation, has
//     only letters, digits, spaces and plain punctuation (no emoji, symbol,
//     bracket, quotation mark or invisible character): the marker discards
//     the match only in a short text
//     (at most 1000 bytes) with no quotation mark (scare quotes: "I
//     'accidentally' cut myself"), bracket or emoticon (";)", ":)", "I
//     (accidentally) cut myself") in which every word outside the marker
//     patterns' own matches
//     comes from a closed benign vocabulary (pronouns and articles, the
//     accident activities, places and objects, times, first aid and pain, the
//     adherence and legal words, particles, laughter, "not a big deal", "nggak
//     sengaja") and that carries no risk signal anywhere, in any sentence: no
//     intent, modal or habit word, no purpose, motive or feeling word
//     ("because", "to feel", "so I", "liked", "deserve", "when I'm sad",
//     "biar", "soalnya", "karena", "rasanya", "lega", "tenang"), no deliberate
//     word ("on purpose", "intentional", "sengaja", "disengaja", "not an
//     accident"), and no method or farewell word ("wrists", "traffic", "the
//     rest", "at once", "wake", "pamit", "selamanya"). So any continuation the
//     vocabulary does not list keeps the match: "I cut myself while cooking
//     because I wanted to", "I cut myself shaving. it helped", "nggak sengaja
//     ngiris tangan pas potong bawang, bikin nagih", "I took all my pills like
//     the doctor said, all 30 of them", "lagi bikin surat wasiat yang sah, udah
//     waktunya" and "I took all my pills on time" are crisis, while "I cut
//     myself shaving this morning" and "I accidentally cut myself while
//     cooking, not a big deal" are none. The accident marker never applies to a
//     habitual or present form ("cutting myself", "I burn myself"), to "sayat"
//     or "silet", to wrists, veins, nadi, an arm or a neck, or after a negating
//     word ("I didn't cut myself accidentally"); a bare "lagi" after the injury
//     is "again", not "while". Ambiguous hyperbole keeps its v1 level ("rasanya
//     mau mati aja kalau disuruh presentasi" is crisis).
//
// # Third-party attribution
//
// Text is attributed to someone else by:
//
//   - Media frames for recorded or published content: v1's "dalam film itu
//     tokoh ..." and "in the movie, the character ...", widened to more media
//     and subjects, and the speaker reading or watching a report about
//     someone else ("aku baca berita ada mahasiswa ...", "I read a news
//     article about a student ..."). Classes and lectures are events the
//     speaker attends and do not frame. In a media frame, song lyrics after a
//     colon included, the report ends at a subjectless wish or thought after
//     the risk phrase ("aku baca berita ada mahasiswa bunuh diri pengen
//     nyusul", "... bunuh diri kepikiran buat ngelakuin hal yang sama",
//     "lirik lagunya: aku ingin mati pengen ngelakuin itu juga": the wish is
//     the speaker's).
//   - Reporting frames at the start of a segment: "temanku bilang ...", "kata
//     dia, ...", "katanya ...", "my friend said ...". "kata" and "menurut"
//     need a third-party subject, since "kata hatiku" and "kata terakhirku"
//     are the speaker's own. The reported content runs to the end of the
//     segment, and on through following segments that open with a
//     third-person pronoun up to the next frame. It stops at a breaker word
//     that starts the speaker's own reaction in subjectless Indonesian
//     ("bikin", "rasanya", "sekarang", "terus", "jadi": "Pacarku bilang putus
//     rasanya pengen mati" is crisis) unless a third-person pronoun is
//     attached ("terus dia mau mati"). Content that opens with a closed
//     report, a quotation (after at most three words such as "sambil
//     marah") or an anaphoric "gitu"/"gt"/"begitu"/"itu" ("Mama bilang
//     \"sabar ya\" pengen bunuh diri", "Nyokap ngomong gitu pengen mati
//     aja"), ends there unless a third-person pronoun follows.
//   - Quotations with a third-party reporting frame right before them (a
//     song's lyrics included: "the lyrics say \"...\""), or an attribution
//     right after them that no first-person word follows before the next
//     comma ("\"...\", she said"). A first-person reporting frame before a
//     quotation ("aku cerita \"...\"", "I told her \"...\"") makes it the
//     speaker's own words whatever follows. A quotation never spans a newline
//     or a sentence boundary, and never ends with a reporting frame after a
//     comma, so a stray quote mark cannot turn the speaker's sentences or
//     clauses into a quotation.
//   - Direct speech after a colon ("temanku bilang: ...", "lirik lagunya:
//     ..."), up to the end of its segment, a first-person reply ("aku jawab
//     ...", "I replied ...") or the start of the speaker's own words after
//     its opening words: a later first-person subject ("Mama bilang: makan
//     dulu aku mau mati"), an adoption form after its opening words ("lirik
//     lagunya: aku ingin mati me too :("), or, unless the speech opens with
//     the quoted person's own "aku", a breaker, "makes me" or a subjectless
//     wish ("makan dulu rasanya pengen mati", "stop crying wanna die", "stop
//     crying want to die", "stop crying makes me want to die").
//   - Third-person wording ("killed himself", "she wants to die"). The
//     speaker's inner self, or a subject that includes the speaker, is not a
//     third person ("part of me", "something inside me", "everything about
//     me", "the real me", "my inner child", "this body", "you and me":
//     crisis), while an object "me" is ("someone
//     close to me wants to die" is elevated). A device or a plant is an idiom
//     ("my laptop wants to die", "the plant next to me wants to die").
//
// Reported content is void when it contains a first-person word outside
// quotation marks, including any -ku form other than a relative or role
// ("temanku bilang aku mau mati" and "kata dia, hidupku ..." are crisis).
// "juga"/"too" or doing "the same" inside indirect reported content is the
// speaker sharing it ("temanku bilang dia mau bunuh diri kepikiran buat
// ngelakuin hal yg sama" is crisis).
//
// # Known limitations
//
// This is a lexicon, not a classifier. It has no acute-violence rules (the
// category exists but, as in v1, nothing uses it), does not recognise
// personal names as third-party subjects, treats advice to others ("jangan
// bunuh diri ya") and some reassurances outside the denial shape ("bukan
// berarti aku mau bunuh diri") as the speaker's own words, keeps v1's
// conservative matches such as "goodbye everyone" and "i have a rope", and
// counts hitting, punching or slapping oneself whatever the purpose ("I
// slapped myself awake", "I hit myself with the door again lol"; only a
// listed accident marker in an otherwise benign text discards it), the
// self-reproach idiom "I could punch myself for ..." (a modal word is never
// discarded), "I hope I can sleep forever this weekend" (the euphemism with
// a time) and a pet before a wish ("kucingku pengen mati": a wish is never
// read as an animal's). A hope or wish in the third person singular ("my
// mom wishes I was dead") is not a rule match, and a free-standing leetspeak
// digit is not read as a letter ("throw myself off 4 bridge"). Some
// third-party reports take the safer path because their extent is ambiguous:
// unquoted direct speech past its first segment ("temanku bilang: aku capek,
// aku mau mati"), quotations of more than one sentence, subjectless content
// after a breaker word, and "<device> aku mau mati" in the middle of a clause
// ("di kosan hp aku mau mati"). "aku sama", "mau ikut", "so am I", "I'm the
// same", "me to", "this is me", "I'm next", "i feel that", "kayak aku" and
// "gw banget" are also ordinary words, so they count as adoption only before
// the trailing words or listed followers above, or when laughter, an
// emoticon or a time adverb closes them: before any other word ("aku sama
// dia", "aku mau ikut nemenin dia", "I'm the same age", "this is me at the
// beach", "I'm next to her") they are not adoption, and the third-party match
// stays elevated. An elliptical wish followed by any content word is read
// as a wish with an object ("aku pengen bantu dia"), also when that word
// starts a new clause. For "aku juga"/"aku pun" the list is the other way
// round, so an unlisted ordinary continuation ("aku juga ikut sedih") takes
// the safer path. A subjectless
// wish that continues indirect reported content with no comma or breaker
// word is read as part of the report ("temanku bilang dia mau bunuh diri
// pengen banget" is elevated; with a comma it is crisis). Context markers
// are only for short messages made of benign words: in a text longer than
// 1000 bytes, one with a quotation mark or any risk signal anywhere, or one
// with any word outside the closed benign vocabulary, an accident, adherence
// or legal-will marker never applies ("Gimana cara bikin surat wasiat yang
// sah? Harus pakai notaris?", "lagi bikin surat wasiat yang sah" and "I cut
// myself while cutting onions" are crisis). A hope not to wake up is read as
// a routine only from the words of its clause, and the text after it is read
// only for a risk word, so a routine that a later sentence names is not one;
// a time with a leetspeak digit fused to it ("at 3am", read as "eam") is not
// a time, so "I hope I dont wake up at 3am" is crisis. An overdose plan
// carries an intent word or a time, so no false-positive check reads it: it
// is crisis whatever the bottle or box holds ("I'm going to take the whole
// bottle of wine to the party", "I want to take the whole pack of gum"),
// "all of them" needs no medicine ("I'll pop all of them in the oven"), and
// a present or future plan to take all one's medicine as prescribed is
// crisis ("I will take all my pills on time from now on", "I'm taking all
// my meds now like the doctor said", "aku mau minum semua obatku sesuai
// resep dokter"); likewise self-harm slang after an intent word is not
// narrowed by its object ("pengen nyilet kertas" is crisis). A hope or wish
// idiom with "I'll" keeps its match ("hopefully I'll get rich and die
// happy"), and being gone, not here or not around after a hope is a death
// wish whatever follows unless a listed everyday occasion, trip or place
// does ("I hope I'm gone before they arrive", "I hope I'm not here when he
// comes" are crisis). A comma segment or clause that "just"/"cuma" opens
// after a denial is read only for a retraction, deferral, condition, means,
// fear, thinking word or wish, so "I'm not suicidal, just sad because my
// cat died" stays a denial while "..., only because of my mom" takes it
// back. Bare "sh" counts as the self-harm slang only after an intent,
// urge, aspect, thought or relapse word, before a relapse, urge or scar
// noun, as a past form ("sh'd"), or after a first-person subject with a
// time, habit or reason word after it ("i sh every night"); with none of
// these around it ("sh be quiet", "run the build.sh file") it is not the
// slang. Fixed idioms are
// read from the words right around the match: a time does not mark
// medication adherence ("I took all
// my meds this morning" is crisis), capital "KMS" cannot be told from slang
// without a card or health-post word, and a leisure jump is only rejected for
// a group or third-party subject ("aku lompat dari atap ke kolam" is
// crisis). The lexical variant resolves elongation and splits fused words
// against the catalogue vocabulary only, and bare "mati" in any spelling is
// not a risk phrase ("m@t1", as v1 "m4t1"). testdata/vectors.json records
// each of these decisions.
package safety

// RulesetVersion identifies the lexicon and routing policy. Change it whenever
// a rule, normalisation step or routing decision changes.
const RulesetVersion = "sela-safety-2026.11.01-1"

// Level is the routing decision.
type Level string

const (
	// LevelNone: continue the ordinary journey.
	LevelNone Level = "none"
	// LevelElevated: continue, but show human-support information alongside
	// (distress without explicit intent, concern about someone else, quoted
	// risk language, ambiguous hyperbole).
	LevelElevated Level = "elevated"
	// LevelCrisis: interrupt. No model call, no handoff, no follow-up plan.
	LevelCrisis Level = "crisis"
)

// Category groups rules for display and evaluation.
type Category string

const (
	CategorySuicide      Category = "suicide"
	CategorySelfHarm     Category = "self_harm"
	CategoryViolence     Category = "acute_violence"
	CategoryHopelessness Category = "extreme_hopelessness"
)

// Assessment is the result of Assess. It never contains the input text.
type Assessment struct {
	Level Level `json:"level"`
	// RuleIDs lists every matched rule id in catalogue order.
	RuleIDs []string `json:"ruleIds,omitempty"`
	// Categories lists the distinct categories of matched rules.
	Categories []Category `json:"categories,omitempty"`
	// ThirdParty is true when the only risk language was attributed to
	// someone else (a friend, a film character, a quotation).
	ThirdParty bool `json:"thirdParty,omitempty"`
	// Denial is true when a clause was recognised as an explicit whole-clause
	// denial ("aku tidak mau bunuh diri") and therefore did not count.
	Denial bool `json:"denial,omitempty"`
	// Ruleset is RulesetVersion at evaluation time.
	Ruleset string `json:"ruleset"`
}

// Interrupts reports whether the ordinary journey must stop.
func (a Assessment) Interrupts() bool { return a.Level == LevelCrisis }

// Max returns the more severe of two assessments, merging rule ids.
func Max(a, b Assessment) Assessment {
	rank := map[Level]int{LevelNone: 0, LevelElevated: 1, LevelCrisis: 2}
	out := a
	if rank[b.Level] > rank[a.Level] {
		out.Level = b.Level
	}
	out.RuleIDs = mergeStrings(a.RuleIDs, b.RuleIDs)
	out.Categories = mergeCategories(a.Categories, b.Categories)
	out.ThirdParty = (a.ThirdParty || a.Level == LevelNone) && (b.ThirdParty || b.Level == LevelNone) && (a.ThirdParty || b.ThirdParty)
	out.Denial = a.Denial || b.Denial
	if out.Ruleset == "" {
		out.Ruleset = RulesetVersion
	}
	return out
}

func mergeStrings(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range append(append([]string{}, a...), b...) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func mergeCategories(a, b []Category) []Category {
	if len(b) == 0 {
		return a
	}
	seen := make(map[Category]bool, len(a)+len(b))
	out := make([]Category, 0, len(a)+len(b))
	for _, c := range append(append([]Category{}, a...), b...) {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}
