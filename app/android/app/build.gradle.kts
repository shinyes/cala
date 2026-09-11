import java.util.Properties
import java.io.FileInputStream

plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

// 正式签名配置从 android/key.properties 读取（该文件不入库）。
//
// 文件缺失时**不报错**，而是回退到 debug 签名，使本地 `flutter run --release`
// 仍然可用。发布路径由 .github/workflows/release.yml 的 preflight 步骤保证
// key.properties 一定存在，且 APK job 会用 apksigner 断言证书不是 debug ——
// 因此「误用 debug 签名发布」不会发生（规格 D18）。
val keystoreProperties = Properties()
val keystorePropertiesFile = rootProject.file("key.properties")
val hasReleaseKeystore = keystorePropertiesFile.exists()
if (hasReleaseKeystore) {
    keystoreProperties.load(FileInputStream(keystorePropertiesFile))
}

android {
    namespace = "cc.lcyk.cala"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        // applicationId 一旦发布即不可更改（改动等于换了一个 App）。
        // 见设计规格 D20。
        applicationId = "cc.lcyk.cala"
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
    }

    signingConfigs {
        if (hasReleaseKeystore) {
            create("release") {
                keyAlias = keystoreProperties["keyAlias"] as String
                keyPassword = keystoreProperties["keyPassword"] as String
                storeFile = (keystoreProperties["storeFile"] as String?)?.let { file(it) }
                storePassword = keystoreProperties["storePassword"] as String
            }
        }
    }

    buildTypes {
        release {
            signingConfig = if (hasReleaseKeystore) {
                signingConfigs.getByName("release")
            } else {
                // 仅供本地便利。CI 的 preflight 会保证发布时不会走到这里。
                signingConfigs.getByName("debug")
            }

            // 不启用 minify/shrink：本项目无反射依赖，且混淆会让崩溃栈难以阅读。
            // Flutter 应用的体积主要在引擎与 Dart AOT 产物，代码压缩收益有限，
            // 不足以抵消调试成本。
            isMinifyEnabled = false
            isShrinkResources = false
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter {
    source = "../.."
}
