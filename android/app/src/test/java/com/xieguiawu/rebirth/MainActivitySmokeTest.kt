package com.xieguiawu.rebirth

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.xieguiawu.rebirth.core.BloodlineData
import com.xieguiawu.rebirth.core.CheckpointData
import com.xieguiawu.rebirth.core.CoreProcess
import com.xieguiawu.rebirth.core.FakeCore
import com.xieguiawu.rebirth.security.InMemorySecureStorage
import com.xieguiawu.rebirth.security.KeystoreSecureStorage
import com.xieguiawu.rebirth.ui.ServiceLocator
import org.junit.AfterClass
import org.junit.BeforeClass
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * Real-framework smoke tests (Robolectric): launch MainActivity with a
 * FakeCore injected through ServiceLocator, verify the Home screen renders
 * the bloodline, navigation to Settings works, and the Create flow opens
 * with drawn births/talents.
 */
@RunWith(AndroidJUnit4::class)
@Config(sdk = [34])
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class MainActivitySmokeTest {

    companion object {
        // Must run before the compose rule launches the Activity (rule
        // before-phase runs ahead of per-test @Before methods).
        @BeforeClass
        @JvmStatic
        fun injectFakes() {
            ServiceLocator.coreClientFactory = {
                FakeCore(
                    bloodline = BloodlineData(generation = 2, sensitivity = 0.55, inheritedTalent = "乐天派"),
                    checkpoint = CheckpointData(exists = true, age = 34, generation = 2),
                )
            }
            ServiceLocator.secureStorageFactory = { InMemorySecureStorage() }
        }

        @AfterClass
        @JvmStatic
        fun resetFakes() {
            ServiceLocator.coreClientFactory = { CoreProcess(it) }
            ServiceLocator.secureStorageFactory = { KeystoreSecureStorage(it) }
        }
    }

    @get:Rule
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Test
    fun home_showsBloodlineAndActions() {
        composeRule.onNodeWithText("重生 Rebirth").assertIsDisplayed()
        composeRule.onNodeWithText("Generation 2").assertIsDisplayed()
        composeRule.onNodeWithText("Start a new life").assertIsDisplayed()
        composeRule.onNodeWithText("Continue life").assertExists()
        composeRule.onNodeWithText("乐天派").assertExists()
    }

    @Test
    fun switchToSettings_showsLanguageAndProviders() {
        composeRule.onNodeWithContentDescription("Settings").performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("Language").assertIsDisplayed()
        composeRule.onNodeWithText("AI narration providers").assertExists()
    }

    @Test
    fun startCreate_opensCreateScreenWithDraws() {
        composeRule.onNodeWithText("Start a new life").performClick()
        composeRule.waitForIdle()
        composeRule.onNodeWithText("Choose your birth").assertIsDisplayed()
        composeRule.onNodeWithText("贫民窟").assertIsDisplayed()
        composeRule.onNodeWithText("Pick 3 talents").assertExists()
        composeRule.onNodeWithText("乐天派").assertExists()
    }
}
