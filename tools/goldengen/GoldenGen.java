import tools.BitTools;
import tools.HexTool;
import tools.MapleAESOFB;
import tools.MapleCustomEncryption;

/**
 * Golden vector generator: runs the ORIGINAL 079MAX2 jar crypto classes on
 * fixed inputs and prints NAME=HEX lines consumed by internal/crypto tests.
 *
 * Compile: javac -cp K:\079MAX2服务端\dist\079MAX2.jar GoldenGen.java
 * Run:     java  -cp .;K:\079MAX2服务端\dist\079MAX2.jar GoldenGen
 */
public class GoldenGen {
    static byte[] fixed(int n) {
        byte[] b = new byte[n];
        for (int i = 0; i < n; i++) b[i] = (byte) (i * 7 + 3);
        return b;
    }

    static void p(String name, byte[] data) {
        System.out.println(name + "=" + HexTool.toString(data).replace(" ", ""));
    }

    public static void main(String[] args) {
        // ---- 1. Shanda (MapleCustomEncryption) ----
        int[] sizes = {1, 8, 16, 100, 1500};
        for (int n : sizes) {
            byte[] buf = fixed(n);
            MapleCustomEncryption.encryptData(buf);
            p("shanda_enc_" + n, buf);
        }
        for (int n : sizes) {
            byte[] buf = fixed(n);
            MapleCustomEncryption.encryptData(buf);
            MapleCustomEncryption.decryptData(buf);
            p("shanda_roundtrip_" + n, buf); // must equal fixed(n)
        }
        // decrypt-only probes (isolate jar decrypt behavior from enc)
        int[] dsizes = {1, 2, 3, 8, 16, 100, 220, 221, 256, 257, 1500};
        for (int n : dsizes) {
            byte[] buf = fixed(n);
            MapleCustomEncryption.decryptData(buf);
            p("shanda_dec_" + n, buf);
        }

        // ---- 2. AES-OFB with receive-style IV/version (79) ----
        byte[] ivRecv = {70, 114, 12, (byte) 199};
        for (int n : sizes) {
            MapleAESOFB ofb = new MapleAESOFB(ivRecv.clone(), (short) 79);
            byte[] buf = fixed(n);
            ofb.crypt(buf);
            p("aesofb79_" + n, buf);
        }
        // IV after crypt (single roll)
        MapleAESOFB ofbA = new MapleAESOFB(ivRecv.clone(), (short) 79);
        ofbA.crypt(fixed(16));
        p("aesofb79_iv_after", ofbA.getIv());

        // ---- 3. AES-OFB with send-style IV/version (-80), like the real server ----
        byte[] ivSend = {82, 48, 120, (byte) 55};
        for (int n : sizes) {
            MapleAESOFB ofb = new MapleAESOFB(ivSend.clone(), (short) -80);
            byte[] buf = fixed(n);
            ofb.crypt(buf);
            p("aesofbSend_" + n, buf);
        }
        // chained crypt: same instance twice (IV rolls between)
        MapleAESOFB ofbB = new MapleAESOFB(ivSend.clone(), (short) -80);
        byte[] b1 = fixed(32);
        ofbB.crypt(b1);
        byte[] b2 = fixed(32);
        ofbB.crypt(b2);
        p("aesofbSend_chain_1", b1);
        p("aesofbSend_chain_2", b2);
        p("aesofbSend_iv_after2", ofbB.getIv());

        // ---- 4. Packet headers (pre-crypt IV) ----
        MapleAESOFB hdr = new MapleAESOFB(ivSend.clone(), (short) -80);
        p("hdr_send_1", hdr.getPacketHeader(1));
        p("hdr_send_100", hdr.getPacketHeader(100));
        p("hdr_send_1500", hdr.getPacketHeader(1500));
        p("hdr_send_0", hdr.getPacketHeader(0));
        MapleAESOFB hdr2 = new MapleAESOFB(ivRecv.clone(), (short) 79);
        p("hdr_recv_100", hdr2.getPacketHeader(100));

        // ---- 5. IV chain via getNewIv ----
        byte[] iv = ivSend.clone();
        for (int i = 1; i <= 5; i++) {
            iv = MapleAESOFB.getNewIv(iv);
            p("newiv_" + i, iv);
        }

        // ---- 6. End-to-end encode (exact encoder pipeline) ----
        // header = crypto.getPacketHeader(len); shanda(data); crypto.crypt(data)
        for (int n : sizes) {
            MapleAESOFB enc = new MapleAESOFB(ivSend.clone(), (short) -80);
            byte[] body = fixed(n);
            byte[] header = enc.getPacketHeader(body.length);
            MapleCustomEncryption.encryptData(body);
            enc.crypt(body);
            byte[] full = new byte[4 + body.length];
            System.arraycopy(header, 0, full, 0, 4);
            System.arraycopy(body, 0, full, 4, body.length);
            p("e2e_" + n, full);
        }

        // ---- 7. checkPacket + getPacketLength cross-check ----
        MapleAESOFB chk = new MapleAESOFB(ivRecv.clone(), (short) 79);
        byte[] h = chk.getPacketHeader(777);
        int asInt = (h[0] & 0xFF) << 24 | (h[1] & 0xFF) << 16 | (h[2] & 0xFF) << 8 | (h[3] & 0xFF);
        System.out.println("chk_777_len=" + MapleAESOFB.getPacketLength(asInt));
        System.out.println("chk_777_ok=" + chk.checkPacket(asInt));
        System.out.println("chk_bad_ok=" + chk.checkPacket(asInt ^ 0xFFFF0000));

        // ---- 8. BitTools rolls (corner cases incl. negative roll counts) ----
        System.out.println("rollLeft_0xAB_3=" + (BitTools.rollLeft((byte) 0xAB, 3) & 0xFF));
        System.out.println("rollRight_0xAB_3=" + (BitTools.rollRight((byte) 0xAB, 3) & 0xFF));
        System.out.println("rollRight_0xAB_neg5=" + (BitTools.rollRight((byte) 0xAB, -5) & 0xFF));
        System.out.println("rollLeft_0xAB_neg3=" + (BitTools.rollLeft((byte) 0xAB, -3) & 0xFF));

        // ---- 9. funnyBytes + key dump (transcription cross-check) ----
        StringBuilder fb = new StringBuilder();
        for (byte b : getFunnyBytes()) fb.append(String.format("%02X", b));
        System.out.println("funnybytes=" + fb);
        StringBuilder kb = new StringBuilder();
        for (byte b : new byte[]{19, 0, 0, 0, 8, 0, 0, 0, 6, 0, 0, 0, -76, 0, 0, 0, 27, 0, 0, 0, 15, 0, 0, 0, 51, 0, 0, 0, 82, 0, 0, 0}) {
            kb.append(String.format("%02X", b));
        }
        System.out.println("key=" + kb);
    }

    // reflection-free copy of the private funnyBytes for the dump
    static byte[] getFunnyBytes() {
        byte[] fb = new byte[256];
        for (int i = 0; i < 256; i++) {
            // derive via funnyShit is not possible; use known table from source
            fb[i] = funnyTable[i];
        }
        return fb;
    }

    static final byte[] funnyTable = new byte[]{
            -20, 63, 119, -92, 69, -48, 113, -65, -73, -104, 32, -4, 75, -23, -77, -31,
            92, 34, -9, 12, 68, 27, -127, -67, 99, -115, -44, -61, -14, 16, 25, -32,
            -5, -95, 110, 102, -22, -82, -42, -50, 6, 24, 78, -21, 120, -107, -37, -70,
            -74, 66, 122, 42, -125, 11, 84, 103, 109, -24, 101, -25, 47, 7, -13, -86,
            39, 123, -123, -80, 38, -3, -117, -87, -6, -66, -88, -41, -53, -52, -110, -38,
            -7, -109, 96, 45, -35, -46, -94, -101, 57, 95, -126, 33, 76, 105, -8, 49,
            -121, -18, -114, -83, -116, 106, -68, -75, 107, 89, 19, -15, 4, 0, -10, 90,
            53, 121, 72, -113, 21, -51, -105, 87, 18, 62, 55, -1, -99, 79, 81, -11,
            -93, 112, -69, 20, 117, -62, -72, 114, -64, -19, 125, 104, -55, 46, 13, 98,
            70, 23, 17, 77, 108, -60, 126, 83, -63, 37, -57, -102, 28, -120, 88, 44,
            -119, -36, 2, 100, 64, 1, 93, 56, -91, -30, -81, 85, -43, -17, 26, 124,
            -89, 91, -90, 111, -122, -97, 115, -26, 10, -34, 43, -103, 74, 71, -100, -33,
            9, 118, -98, 48, 14, -28, -78, -108, -96, 59, 52, 29, 40, 15, 54, -29,
            35, -76, 3, -40, -112, -56, 60, -2, 94, 50, 36, 80, 31, 58, 67, -118,
            -106, 65, 116, -84, 82, 51, -16, -39, 41, -128, -79, 22, -45, -85, -111, -71,
            -124, 127, 97, 30, -49, -59, -47, 86, 61, -54, -12, 5, -58, -27, 8, 73,
    };
}
