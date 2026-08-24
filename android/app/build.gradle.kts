plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.serialization")
    id("org.jetbrains.kotlin.plugin.compose")
}

import java.util.Properties

// Optional local signing for side-loading and GitHub Release artifacts.
// F-Droid rebuilds with its own signature (or the Verified route uses the
// AllowedAPKSigningKeys below); keystore.properties is never committed.
val ksFile = rootProject.file("keystore.properties")
val ksProps = Properties()
if (ksFile.exists()) {
    ksFile.inputStream().use { ksProps.load(it) }
}

android {
    namespace = "com.xieguiawu.rebirth"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.xieguiawu.rebirth"
        minSdk = 26
        targetSdk = 35
        versionCode = 1000
        versionName = "0.10.0"
    }

    signingConfigs {
        if (ksFile.exists()) {
            create("release") {
                storeFile = file(ksProps["storeFile"] as String)
                storePassword = ksProps["storePassword"] as String
                keyAlias = ksProps["keyAlias"] as String
                keyPassword = ksProps["keyPassword"] as String
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            if (ksFile.exists()) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = "17" }
    buildFeatures { compose = true }

    // The Go core is a static PIE ELF renamed to librebirth_core.so.
    // It must be extracted (not mmap'd) so it can be exec'd from nativeLibraryDir.
    packaging {
        jniLibs {
            useLegacyPackaging = true
        }
    }

    testOptions {
        unitTests.isReturnDefaultValues = true
        unitTests.isIncludeAndroidResources = true
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2024.10.00")
    implementation(composeBom)
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.activity:activity-compose:1.9.2")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.6")
    implementation("androidx.compose.material3:material3")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.9.0")
    testImplementation("org.robolectric:robolectric:4.14.1")
    testImplementation("androidx.test.ext:junit:1.2.1")
    testImplementation("androidx.test:core:1.6.1")
    testImplementation("androidx.compose.ui:ui-test-junit4")
    testImplementation("androidx.compose.ui:ui-test-manifest")
}

tasks.withType<Test>().configureEach {
    jvmArgs("-Dfile.encoding=UTF-8")
}
