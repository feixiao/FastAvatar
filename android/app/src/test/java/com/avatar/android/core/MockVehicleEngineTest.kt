package com.avatar.android.core

import com.avatar.android.core.mockvehicle.DecisionType
import com.avatar.android.core.mockvehicle.MockVehicleEngine
import com.avatar.android.core.mockvehicle.ReasonCodes
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.io.File

class MockVehicleEngineTest {

    private lateinit var tempFile: File
    private lateinit var engine: MockVehicleEngine

    @Before
    fun setUp() {
        tempFile = File.createTempFile("test_vehicle_audit", ".log")
        engine = MockVehicleEngine(tempFile)
    }

    @After
    fun tearDown() {
        tempFile.delete()
    }

    @Test
    fun testClimateSetTemperature_ValidSafeRange_Accepted() {
        val slots = mapOf(
            "target_temperature" to 24.0,
            "zone" to "driver"
        )
        val res = engine.execute("climate.set_temperature", slots)

        assertTrue(res.accepted)
        assertEquals(DecisionType.MOCK_ACCEPTED, res.entry.decision)
        assertEquals(ReasonCodes.VALID_SLOTS_AND_SAFE_RANGE, res.entry.reason)
        assertTrue(res.entry.simulation)
        assertTrue(res.confirmation.contains("24.0"))
    }

    @Test
    fun testClimateSetTemperature_OutOfRange_Rejected() {
        val slots = mapOf(
            "target_temperature" to 35.0,
            "zone" to "driver"
        )
        val res = engine.execute("climate.set_temperature", slots)

        assertFalse(res.accepted)
        assertEquals(DecisionType.MOCK_REJECTED, res.entry.decision)
        assertEquals(ReasonCodes.ERR_TEMP_OUT_OF_RANGE, res.entry.reason)
    }

    @Test
    fun testClimateSetTemperature_MissingSlot_Rejected() {
        val slots = emptyMap<String, Any?>()
        val res = engine.execute("climate.set_temperature", slots)

        assertFalse(res.accepted)
        assertEquals(DecisionType.MOCK_REJECTED, res.entry.decision)
        assertEquals(ReasonCodes.ERR_SLOT_MISSING, res.entry.reason)
    }

    @Test
    fun testWindowControl_ValidActions_Accepted() {
        val openSlots = mapOf("action" to "open", "position" to "driver")
        val res1 = engine.execute("window.control", openSlots)
        assertTrue(res1.accepted)
        assertEquals(DecisionType.MOCK_ACCEPTED, res1.entry.decision)

        val ventSlots = mapOf("action" to "vent", "position" to "all")
        val res2 = engine.execute("window.control", ventSlots)
        assertTrue(res2.accepted)
        assertEquals(DecisionType.MOCK_ACCEPTED, res2.entry.decision)
    }

    @Test
    fun testWindowControl_UnsupportedAction_Rejected() {
        val slots = mapOf("action" to "smash", "position" to "driver")
        val res = engine.execute("window.control", slots)
        assertFalse(res.accepted)
        assertEquals(DecisionType.MOCK_REJECTED, res.entry.decision)
        assertEquals(ReasonCodes.ERR_UNSUPPORTED_WINDOW_ACTION, res.entry.reason)
    }

    @Test
    fun testDriveMode_SafeModes_Accepted() {
        val safeModes = listOf("eco", "sport", "comfort", "standard")
        for (m in safeModes) {
            val res = engine.execute("drive.switch_mode", mapOf("mode" to m))
            assertTrue("Expected accepted for mode $m", res.accepted)
            assertEquals(DecisionType.MOCK_ACCEPTED, res.entry.decision)
        }
    }

    @Test
    fun testDriveMode_DangerousModes_StrictlyRejected() {
        val dangerousModes = listOf("drift", "race", "track", "offroad")
        for (m in dangerousModes) {
            val res = engine.execute("drive.switch_mode", mapOf("mode" to m))
            assertFalse("Expected rejected for dangerous mode $m", res.accepted)
            assertEquals(DecisionType.MOCK_REJECTED, res.entry.decision)
            assertEquals(ReasonCodes.ERR_DANGEROUS_ACTION_REJECTED, res.entry.reason)
        }
    }

    @Test
    fun testAuditLogFormat_ZeroAudioOrTranscriptLeaks() {
        engine.execute("climate.set_temperature", mapOf("target_temperature" to 22.0))
        engine.execute("drive.switch_mode", mapOf("mode" to "drift"))

        val lines = tempFile.readLines()
        assertEquals(2, lines.size)

        for (line in lines) {
            // Must contain simulation: true
            assertTrue(line.contains("\"simulation\":true"))
            // Must contain UTC RFC 3339 timestamp with Z
            assertTrue(line.contains("Z\""))
            // Must NOT contain raw audio or transcripts
            assertFalse(line.contains("audio"))
            assertFalse(line.contains("raw_text"))
            assertFalse(line.contains("transcript"))
        }
    }
}
