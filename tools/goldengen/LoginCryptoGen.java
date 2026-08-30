import client.LoginCrypto;
import client.LoginCryptoLegacy;

public class LoginCryptoGen {
    public static void main(String[] args) {
        String pw = "goldenpassword";
        String salt = "0123456789abcdef0123456789abcdef";
        System.out.println("sha1_" + pw + "=" + LoginCrypto.hexSha1(pw));
        System.out.println("sha1_empty=" + LoginCrypto.hexSha1(""));
        System.out.println("sha1_中文=" + LoginCrypto.hexSha1("中文"));
        System.out.println("sha512_" + pw + "_" + salt + "=" + LoginCrypto.makeSaltedSha512Hash(pw, salt));
        System.out.println("sha512_empty_salt=" + LoginCrypto.makeSaltedSha512Hash("", salt));
        String legacy = LoginCryptoLegacy.hashPassword(pw);
        System.out.println("legacy_hash=" + legacy);
        System.out.println("legacy_pw=" + pw);
        System.out.println("legacy_check_ok=" + LoginCryptoLegacy.checkPassword(pw, legacy));
        System.out.println("legacy_check_bad=" + LoginCryptoLegacy.checkPassword("wrong", legacy));
        System.out.println("legacy_is_legacy=" + LoginCryptoLegacy.isLegacyPassword(legacy));
    }
}
